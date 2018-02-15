package defaults

import (
	"github.com/banzaicloud/banzai-types/constants"
	"github.com/banzaicloud/pipeline/model"
	"github.com/banzaicloud/banzai-types/components"
	"github.com/banzaicloud/banzai-types/components/amazon"
	"github.com/banzaicloud/banzai-types/components/azure"
	"github.com/banzaicloud/banzai-types/components/google"
)

// todo maybe this could be private
type AWSProfile struct {
	DefaultModel
	Location           string `gorm:"default:'eu-west-1'"`
	NodeInstanceType   string `gorm:"default:'m4.xlarge'"`
	NodeImage          string `gorm:"default:'ami-06d1667f'"`
	MasterInstanceType string `gorm:"default:'m4.xlarge'"`
	MasterImage        string `gorm:"default:'ami-06d1667f'"`
	NodeSpotPrice      string `gorm:"default:'0.2'"`
	NodeMinCount       int    `gorm:"default:1"`
	NodeMaxCount       int    `gorm:"default:2"`
}

func (*AWSProfile) TableName() string {
	return defaultAmazonProfileTablaName
}

func (d *AWSProfile) SaveInstance() error {
	return save(d)
}

func (d *AWSProfile) GetType() string {
	return constants.Amazon
}

func (d *AWSProfile) IsDefinedBefore() bool {
	return model.GetDB().First(&d).RowsAffected != int64(0)
}

func (d *AWSProfile) GetProfile() *components.ClusterProfileRespone {
	loadFirst(&d)

	return &components.ClusterProfileRespone{
		ProfileName:      d.DefaultModel.Name,
		Location:         d.Location,
		Cloud:            constants.Amazon,
		NodeInstanceType: d.NodeInstanceType,
		Properties: struct {
			Amazon *amazon.ClusterProfileAmazon `json:"amazon,omitempty"`
			Azure  *azure.ClusterProfileAzure   `json:"azure,omitempty"`
			Google *google.ClusterProfileGoogle `json:"google,omitempty"`
		}{
			Amazon: &amazon.ClusterProfileAmazon{
				Node: &amazon.AmazonProfileNode{
					SpotPrice: d.NodeSpotPrice,
					MinCount:  d.NodeMinCount,
					MaxCount:  d.NodeMaxCount,
					Image:     d.NodeImage,
				},
				Master: &amazon.AmazonProfileMaster{
					InstanceType: d.MasterInstanceType,
					Image:        d.MasterImage,
				},
			},
		},
	}

}

func (d *AWSProfile) UpdateProfile(r *components.ClusterProfileRequest) error {

	if len(r.Location) != 0 {
		d.Location = r.Location
	}

	if len(r.NodeInstanceType) != 0 {
		d.NodeInstanceType = r.NodeInstanceType
	}
	if r.Properties.Amazon != nil {
		if r.Properties.Amazon.Node != nil {
			if len(r.Properties.Amazon.Node.SpotPrice) != 0 {
				d.NodeSpotPrice = r.Properties.Amazon.Node.SpotPrice
			}

			if r.Properties.Amazon.Node.MinCount != 0 {
				d.NodeMinCount = r.Properties.Amazon.Node.MinCount
			}

			if r.Properties.Amazon.Node.MaxCount != 0 {
				d.NodeMaxCount = r.Properties.Amazon.Node.MaxCount
			}

			if len(r.Properties.Amazon.Node.Image) != 0 {
				d.NodeImage = r.Properties.Amazon.Node.Image
			}
		}

		if r.Properties.Amazon.Master != nil {
			if len(r.Properties.Amazon.Master.InstanceType) != 0 {
				d.MasterInstanceType = r.Properties.Amazon.Master.InstanceType
			}

			if len(r.Properties.Amazon.Master.Image) != 0 {
				d.MasterImage = r.Properties.Amazon.Master.Image
			}
		}
	}

	return d.SaveInstance()
}

func (d *AWSProfile) DeleteProfile() error {
	return model.GetDB().Delete(&d).Error
}
