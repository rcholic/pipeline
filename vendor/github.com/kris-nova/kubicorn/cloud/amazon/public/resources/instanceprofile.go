package resources

import (
	"github.com/kris-nova/kubicorn/cloud"
	"github.com/kris-nova/kubicorn/apis/cluster"
	"github.com/kris-nova/kubicorn/cutil/logger"
	"github.com/kris-nova/kubicorn/cutil/compare"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/kris-nova/kubicorn/cutil/defaults"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"net/url"
)

var _ cloud.Resource = &InstanceProfile{}

type InstanceProfile struct {
	Shared
	Role  *IAMRole
	ServerPool *cluster.ServerPool
}

type IAMRole struct {
	Shared
	Policies []*IAMPolicy
}

type IAMPolicy struct {
	Shared
	Document  string
}



func (r *InstanceProfile) Actual(immutable *cluster.Cluster) (*cluster.Cluster, cloud.Resource, error) {
	logger.Debug("instanceprofile.Actual")
	newResource := &InstanceProfile{
		Shared: Shared{
			Name: r.Name,
			Tags: map[string]string{
				"Name":                           r.Name,
				"KubernetesCluster":              immutable.Name,
			},
		},
		ServerPool: r.ServerPool,
	}
	// Get InstanceProfile
	if r.Identifier != "" {
		logger.Debug("Query InstanceProfile: %v", newResource.Name)
		respInstanceProfile, err := Sdk.IAM.GetInstanceProfile(&iam.GetInstanceProfileInput{
			InstanceProfileName: &newResource.Name,
		})
		if err != nil {
			return nil, nil, err
		}
		newResource.Identifier = *respInstanceProfile.InstanceProfile.InstanceProfileName
		// Get Roles
		if len(respInstanceProfile.InstanceProfile.Roles) > 0 {
			//ListRolePolicies
			for _, role := range respInstanceProfile.InstanceProfile.Roles {
				policyList, err := Sdk.IAM.ListRolePolicies( &iam.ListRolePoliciesInput{
					RoleName: role.RoleName,
				})
				if err != nil {
					return nil, nil, err
				}
				//Here we add the role to InstanceProfile
				iamrole := &IAMRole{
					Shared: Shared{
						Tags: map[string]string{
							"Name":                           r.Name,
							"KubernetesCluster":              immutable.Name,
						},
						Name: *role.RoleName,
					},
				}
				newResource.Role = iamrole
				
				for _, policyName := range policyList.PolicyNames {
					policyOutput, err := Sdk.IAM.GetRolePolicy(&iam.GetRolePolicyInput{
						PolicyName: policyName,
						RoleName: role.RoleName,
					})
					if err != nil {
						return nil, nil, err
					}
					//Here we add the policy to the role
					iampolicy := &IAMPolicy{
						Shared: Shared{
							Tags: map[string]string{
								"Name":                           r.Name,
								"KubernetesCluster":              immutable.Name,
							},
							Name: *policyOutput.PolicyName,
						},
					}
					raw, err := url.QueryUnescape(*policyOutput.PolicyDocument)
					if err != nil {
						return nil, nil, err
					}
					iampolicy.Document = raw
					iamrole.Policies = append(iamrole.Policies, iampolicy)
				}
			}
		}
	}
	newCluster := r.immutableRender(newResource, immutable)
	return newCluster, newResource, nil
}

func (r *InstanceProfile) Expected(immutable *cluster.Cluster) (*cluster.Cluster, cloud.Resource, error) {
	logger.Debug("instanceprofile.Expected %v", r.Name)
	newResource := &InstanceProfile{
		Shared: Shared{
			Tags: map[string]string{
				"Name":                           r.Name,
				"KubernetesCluster":              immutable.Name,
				},
			Name:   r.Name,
			Identifier: r.Identifier,
		},
		ServerPool: r.ServerPool,
		Role: &IAMRole{
			Shared: Shared{
				Name: r.Role.Name,
				Tags: map[string]string{
					"Name":                           r.Name,
					"KubernetesCluster":              immutable.Name,
				},
			},
			Policies: []*IAMPolicy{},
		},
	}
	for _, policy := range r.Role.Policies {
		newPolicy := &IAMPolicy{
			Shared: Shared{
				Name: policy.Name,
				Tags: map[string]string{
					"Name":                           r.Name,
					"KubernetesCluster":              immutable.Name,
				},
			},
			Document: policy.Document,
		}
		newResource.Role.Policies = append(newResource.Role.Policies, newPolicy)
	}
	newCluster := r.immutableRender(newResource, immutable)
	return newCluster, newResource, nil
}

func (r *InstanceProfile) Apply(actual, expected cloud.Resource, immutable *cluster.Cluster) (*cluster.Cluster, cloud.Resource, error) {
	logger.Debug("instanceprofile.Apply")
	applyResource := expected.(*InstanceProfile)
	isEqual, err := compare.IsEqual(actual.(*InstanceProfile), expected.(*InstanceProfile))
	if err != nil {
			return nil, nil, err
	}
	if isEqual {
		return immutable, applyResource, nil
	}
	logger.Debug("Actual: %#v", actual)
	logger.Debug("Expectd: %#v", expected)
	newResource := &InstanceProfile{}
	//TODO fill in instanceprofile attributes
	
	// Create InstanceProfile
	profileinput := &iam.CreateInstanceProfileInput{
		InstanceProfileName: &expected.(*InstanceProfile).Name,
		Path: S("/"),
	}
	outInstanceProfile, err := Sdk.IAM.CreateInstanceProfile(profileinput)
	if err != nil{
		logger.Debug("CreateInstanceProfile error: %v", err)
		if err.(awserr.Error).Code() != iam.ErrCodeEntityAlreadyExistsException {
			return nil, nil, err
		}
	}
	newResource.Name = *outInstanceProfile.InstanceProfile.InstanceProfileName
	newResource.Identifier = *outInstanceProfile.InstanceProfile.InstanceProfileName
	logger.Info("InstanceProfile created: %s", newResource.Name)
	// Create role
	assumeRolePolicy := `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "ec2.amazonaws.com"
            },
            "Action": "sts:AssumeRole"
        }
    ]}`
	roleinput := &iam.CreateRoleInput{
		AssumeRolePolicyDocument: &assumeRolePolicy,
		RoleName: &expected.(*InstanceProfile).Role.Name,
		Description: S("Kubicorn Role"),
		Path: S("/"),
	}
	outInstanceRole, err := Sdk.IAM.CreateRole(roleinput)
	if err != nil{
		logger.Debug("CreateRole error: %v", err)
		if err.(awserr.Error).Code() != iam.ErrCodeEntityAlreadyExistsException {
			return nil, nil, err
		}
	}
	newIamRole := &IAMRole{
		Shared: Shared{
			Name: *outInstanceRole.Role.RoleName,
			Tags: map[string]string{
				"Name":                           r.Name,
				"KubernetesCluster":              immutable.Name,
			},
		},
		Policies: []*IAMPolicy{},
	}
	logger.Info("Role created")
	//Attach Policy to Role
	for _, policy := range expected.(*InstanceProfile).Role.Policies {
		policyinput := &iam.PutRolePolicyInput{
			PolicyDocument: &policy.Document,
			PolicyName: &policy.Name,
			RoleName: &expected.(*InstanceProfile).Role.Name,
		}
		_, err := Sdk.IAM.PutRolePolicy(policyinput)
		if err != nil{
			logger.Debug("PutRolePolicy error: %v", err)
			if err.(awserr.Error).Code() != iam.ErrCodeLimitExceededException {
				return nil, nil, err
			}
		}
		newPolicy := &IAMPolicy{
			Shared: Shared{
				Name: policy.Name,
				Tags: map[string]string{
					"Name":                           r.Name,
					"KubernetesCluster":              immutable.Name,
				},
			},
			Document: policy.Document,
		}
		newIamRole.Policies = append(newIamRole.Policies, newPolicy)
		logger.Info("Policy created")
	}
	//Attach Role to Profile
	roletoprofile := &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: &expected.(*InstanceProfile).Name,
		RoleName: &expected.(*InstanceProfile).Role.Name,
	}
	_, err = Sdk.IAM.AddRoleToInstanceProfile(roletoprofile)
	if err != nil{
		logger.Debug("AddRoleToInstanceProfile error: %v", err)
		if err.(awserr.Error).Code() != iam.ErrCodeLimitExceededException {
			return nil, nil, err
		}
	}
	newResource.Role = newIamRole
	logger.Info("RoleAttachedToInstanceProfile done")
	//Add ServerPool
	newResource.ServerPool = expected.(*InstanceProfile).ServerPool
	newCluster := r.immutableRender(newResource, immutable)
	return newCluster, newResource, nil
}

func (r *InstanceProfile) Delete(actual cloud.Resource, immutable *cluster.Cluster) (*cluster.Cluster, cloud.Resource, error) {
	for _, policy := range r.Role.Policies{
		_, err := Sdk.IAM.DeleteRolePolicy(&iam.DeleteRolePolicyInput{
			PolicyName: &policy.Name,
			RoleName: &r.Role.Name,
		})
		if err != nil {
			logger.Debug("Problem deleting Policy %s for Role: %s: %v", policy.Name, r.Role.Name, err )
			if err.(awserr.Error).Code() != iam.ErrCodeNoSuchEntityException {
				return nil, nil, err
			}
		}
	}
	_, err := Sdk.IAM.RemoveRoleFromInstanceProfile(&iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: &r.Name,
		RoleName: &r.Role.Name,
	})
	if err != nil {
		logger.Debug("Problem remove Role %s from InstanceProfile %s: %v", r.Role.Name, r.Name, err  )
		if err.(awserr.Error).Code() != iam.ErrCodeNoSuchEntityException {
			return nil, nil, err
		}
	}
	_, err = Sdk.IAM.DeleteRole(&iam.DeleteRoleInput{
		RoleName: &r.Role.Name,
	})
	if err != nil {
		logger.Debug("Problem delete role %s: %v", r.Role.Name, err )
		if err.(awserr.Error).Code() != iam.ErrCodeNoSuchEntityException {
			return nil, nil, err
		}
	}
	_, err = Sdk.IAM.DeleteInstanceProfile(&iam.DeleteInstanceProfileInput{
		InstanceProfileName:    &r.Name,
	})
	if err != nil {
		logger.Debug("Problem delete InstanceProfile %s: %v", r.Name, err )
		if err.(awserr.Error).Code() != iam.ErrCodeNoSuchEntityException {
			return nil, nil, err
		}
	}
	newResource := &InstanceProfile{}
	newCluster := r.immutableRender(newResource, immutable)
	logger.Info("Deleted InstanceProfile: %v", r.Name)
	return newCluster, newResource, nil
}

func (r *InstanceProfile) immutableRender(newResource cloud.Resource, inaccurateCluster *cluster.Cluster) *cluster.Cluster {
	logger.Debug("instanceprofile.Render")
	newCluster := defaults.NewClusterDefaults(inaccurateCluster)
	instanceProfile := &cluster.IAMInstanceProfile{}
	instanceProfile.Name = newResource.(*InstanceProfile).Name
	instanceProfile.Identifier = newResource.(*InstanceProfile).Identifier
	instanceProfile.Role = &cluster.IAMRole{}
	if newResource.(*InstanceProfile).Role != nil {
		instanceProfile.Role.Name = newResource.(*InstanceProfile).Role.Name
		if len(newResource.(*InstanceProfile).Role.Policies) > 0 {
			for i := range newResource.(*InstanceProfile).Role.Policies {
				newPolicy := &cluster.IAMPolicy{
					Name: newResource.(*InstanceProfile).Role.Policies[i].Name,
					Identifier: newResource.(*InstanceProfile).Role.Policies[i].Identifier,
					Document: newResource.(*InstanceProfile).Role.Policies[i].Document,
				}
				instanceProfile.Role.Policies = append(instanceProfile.Role.Policies, newPolicy)
			}
		}
	}
	found := false
	for i := 0; i < len(newCluster.ServerPools); i++ {
		if newResource.(*InstanceProfile).ServerPool != nil && newCluster.ServerPools[i].Name == newResource.(*InstanceProfile).ServerPool.Name {
			newCluster.ServerPools[i].InstanceProfile = instanceProfile
			found = true
			}
	}
	if !found {
		logger.Debug("Not found ServerPool for InstanceProfile!")
	}
	return newCluster
}