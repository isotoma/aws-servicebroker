package broker

import (
	"errors"
	"io/ioutil"
	"log"
	"strings"
	"testing"

	"github.com/awslabs/aws-service-broker/pkg/serviceinstance"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/koding/cache"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

type TestCases map[string]Options

func (T *TestCases) GetTests(f string) error {
	yamlFile, err := ioutil.ReadFile(f)
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
		return err
	}
	err = yaml.Unmarshal(yamlFile, &T)
	if err != nil {
		log.Printf("Unmarshal: %v", err)
		return err
	}
	return nil
}

func mockGetAwsSession(keyid string, secretkey string, region string, accountId string, profile string, params map[string]string) *session.Session {
	sess := mock.Session
	conf := aws.NewConfig()
	conf.Region = aws.String(region)
	return sess.Copy(conf)
}

func mockAwsCfnClientGetter(sess *session.Session) CfnClient {
	return CfnClient{mockCfn{
		DescribeStacksResponse: cloudformation.DescribeStacksOutput{},
	}}
}

func mockAwsStsClientGetter(sess *session.Session) *sts.STS {
	conf := aws.NewConfig()
	conf.Region = sess.Config.Region
	return &sts.STS{Client: mock.NewMockClient(conf)}
}

func mockAwsS3ClientGetter(sess *session.Session) S3Client {
	conf := aws.NewConfig()
	conf.Region = sess.Config.Region
	return S3Client{s3iface.S3API(&s3.S3{Client: mock.NewMockClient(conf)})}
}

func mockAwsDdbClientGetter(sess *session.Session) *dynamodb.DynamoDB {
	conf := aws.NewConfig()
	conf.Region = sess.Config.Region
	return &dynamodb.DynamoDB{Client: mock.NewMockClient(conf)}
}

func mockAwsSsmClientGetter(sess *session.Session) *ssm.SSM {
	conf := aws.NewConfig()
	conf.Region = sess.Config.Region
	return &ssm.SSM{Client: mock.NewMockClient(conf)}
}

var mockClients = AwsClients{
	NewCfn: mockAwsCfnClientGetter,
	NewSsm: mockAwsSsmClientGetter,
	NewS3:  mockAwsS3ClientGetter,
	NewDdb: mockAwsDdbClientGetter,
	NewSts: mockAwsStsClientGetter,
}

func mockGetAccountId(svc stsiface.STSAPI) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{Account: aws.String("123456789012")}, nil
}

func mockGetAccountIdFail(svc stsiface.STSAPI) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{}, errors.New("I should be failing...")
}

func mockUpdateCatalog(listingcache cache.Cache, catalogcache cache.Cache, bd BucketDetailsRequest, s3svc S3Client, db Db, bl AwsBroker, listTemplates ListTemplateser, listingUpdate ListingUpdater, metadataUpdate MetadataUpdater) error {
	return nil
}

func mockUpdateCatalogFail(listingcache cache.Cache, catalogcache cache.Cache, bd BucketDetailsRequest, s3svc S3Client, db Db, bl AwsBroker, listTemplates ListTemplateser, listingUpdate ListingUpdater, metadataUpdate MetadataUpdater) error {
	return errors.New("I failed")
}

func mockPollUpdate(interval int, l cache.Cache, c cache.Cache, bd BucketDetailsRequest, s3svc S3Client, db Db, bl AwsBroker, updateCatalog UpdateCataloger, listTemplates ListTemplateser) {

}

// mock implementation of DataStore Adapter
type mockDataStore struct{}

func (db mockDataStore) PutServiceDefinition(sd osb.Service) error                   { return nil }
func (db mockDataStore) GetParam(paramname string) (value string, err error)         { return "some-value", nil }
func (db mockDataStore) PutParam(paramname string, paramvalue string) error          { return nil }
func (db mockDataStore) PutServiceInstance(si serviceinstance.ServiceInstance) error { return nil }
func (db mockDataStore) GetServiceDefinition(serviceuuid string) (*osb.Service, error) {
	service := osb.Service{
		ID:                  "",
		Name:                "",
		Description:         "",
		Tags:                nil,
		Requires:            nil,
		Bindable:            false,
		BindingsRetrievable: false,
		PlanUpdatable:       nil,
		Plans:               nil,
		DashboardClient: &osb.DashboardClient{
			ID:          "",
			Secret:      "",
			RedirectURI: "",
		},
		Metadata: nil,
	}
	return &service, nil
}
func (db mockDataStore) GetServiceInstance(sid string) (*serviceinstance.ServiceInstance, error) {
	si := serviceinstance.ServiceInstance{
		ID:        "",
		ServiceID: "",
		PlanID:    "",
		Params:    nil,
		StackID:   "",
	}
	return &si, nil
}
func (db mockDataStore) GetServiceBinding(id string) (*serviceinstance.ServiceBinding, error) {
	return nil, nil
}
func (db mockDataStore) PutServiceBinding(sb serviceinstance.ServiceBinding) error { return nil }
func (db mockDataStore) DeleteServiceBinding(id string) error                      { return nil }

func TestNewAwsBroker(t *testing.T) {
	assert := assert.New(t)
	options := new(TestCases)
	options.GetTests("../../testcases/options.yaml")

	for _, v := range *options {
		// Shouldn't error
		bl, err := NewAWSBroker(v, mockGetAwsSession, mockClients, mockGetAccountId, mockUpdateCatalog, mockPollUpdate)
		assert.Nil(err)

		// check values are as expected
		assert.Equal(v.KeyID, bl.keyid)
		assert.Equal(v.SecretKey, bl.secretkey)
		assert.Equal(v.Profile, bl.secretkey)
		assert.Equal(v.Profile, bl.profile)
		assert.Equal(v.TableName, bl.tablename)
		assert.Equal(v.S3Bucket, bl.s3bucket)
		assert.Equal(v.S3Region, bl.s3region)
		assert.Equal(AddTrailingSlash(v.S3Key), bl.s3key)
		assert.Equal(v.TemplateFilter, bl.templatefilter)
		assert.Equal(v.Region, bl.region)
		assert.Equal(v.BrokerID, bl.brokerid)
		assert.Equal("123456789012", bl.db.Accountid)
		assert.Equal(uuid.NewV5(uuid.NullUUID{}.UUID, "123456789012"+v.BrokerID), bl.db.Accountuuid)
		assert.Equal(v.BrokerID, bl.db.Brokerid)

		// Should error
		_, err = NewAWSBroker(v, mockGetAwsSession, mockClients, mockGetAccountIdFail, mockUpdateCatalog, mockPollUpdate)
		assert.Error(err)

		// Should error
		_, err = NewAWSBroker(v, mockGetAwsSession, mockClients, mockGetAccountId, mockUpdateCatalogFail, mockPollUpdate)
		assert.Error(err)
	}
}

func mockListTemplates(s3source *BucketDetailsRequest, b *AwsBroker) (*[]ServiceLastUpdate, error) {
	return &[]ServiceLastUpdate{}, nil
}

func mockListTemplatesFailNoBucket(s3source *BucketDetailsRequest, b *AwsBroker) (*[]ServiceLastUpdate, error) {
	return &[]ServiceLastUpdate{}, errors.New("NoSuchBucket: The specified bucket does not exist")
}

func mockListTemplatesFail(s3source *BucketDetailsRequest, b *AwsBroker) (*[]ServiceLastUpdate, error) {
	return &[]ServiceLastUpdate{}, errors.New("ListTemplates failed")
}

func mockListingUpdate(l *[]ServiceLastUpdate, c cache.Cache) error {
	return nil
}

func mockListingUpdateFail(l *[]ServiceLastUpdate, c cache.Cache) error {
	return errors.New("ListingUpdate failed")
}

func mockMetadataUpdate(l cache.Cache, c cache.Cache, bd BucketDetailsRequest, s3svc S3Client, db Db, metadataUpdate MetadataUpdater) error {
	return nil
}

func mockMetadataUpdateFail(l cache.Cache, c cache.Cache, bd BucketDetailsRequest, s3svc S3Client, db Db, metadataUpdate MetadataUpdater) error {
	return errors.New("MetadataUpdate failed")
}

func TestUpdateCatalog(t *testing.T) {
	assert := assert.New(t)
	options := new(TestCases)
	options.GetTests("../../testcases/options.yaml")
	var bl *AwsBroker
	var bd *BucketDetailsRequest
	for _, v := range *options {
		bl, _ = NewAWSBroker(v, mockGetAwsSession, mockClients, mockGetAccountId, mockUpdateCatalog, mockPollUpdate)
		bd = &BucketDetailsRequest{
			v.S3Bucket,
			v.S3Key,
			v.TemplateFilter,
		}
	}

	bl.db.DataStorePort = mockDataStore{}

	err := UpdateCatalog(bl.listingcache, bl.catalogcache, *bd, bl.s3svc, bl.db, *bl, mockListTemplates, mockListingUpdate, mockMetadataUpdate)
	assert.Nil(err)

	err = UpdateCatalog(bl.listingcache, bl.catalogcache, *bd, bl.s3svc, bl.db, *bl, mockListTemplatesFailNoBucket, mockListingUpdate, mockMetadataUpdate)
	assert.EqualError(err, "Cannot access S3 Bucket, either it does not exist or the IAM user/role the broker is configured to use has no access to the bucket")

	err = UpdateCatalog(bl.listingcache, bl.catalogcache, *bd, bl.s3svc, bl.db, *bl, mockListTemplatesFail, mockListingUpdate, mockMetadataUpdate)
	assert.EqualError(err, "ListTemplates failed")

	err = UpdateCatalog(bl.listingcache, bl.catalogcache, *bd, bl.s3svc, bl.db, *bl, mockListTemplates, mockListingUpdateFail, mockMetadataUpdate)
	assert.EqualError(err, "ListingUpdate failed")

	err = UpdateCatalog(bl.listingcache, bl.catalogcache, *bd, bl.s3svc, bl.db, *bl, mockListTemplates, mockListingUpdate, mockMetadataUpdateFail)
	assert.EqualError(err, "MetadataUpdate failed")
}

type mockS3 struct {
	s3iface.S3API
	GetObjectResp s3.GetObjectOutput
}

func (m mockS3) GetObject(in *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	return &m.GetObjectResp, nil
}

type mockCfn struct {
	cloudformationiface.CloudFormationAPI
	DescribeStacksResponse cloudformation.DescribeStacksOutput
	CreateStackResponse    cloudformation.CreateStackOutput
	DeleteStackResponse    cloudformation.DeleteStackOutput
}

func (m mockCfn) DescribeStacks(in *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error) {
	return &m.DescribeStacksResponse, nil
}

func (m mockCfn) CreateStack(in *cloudformation.CreateStackInput) (*cloudformation.CreateStackOutput, error) {
	return &m.CreateStackResponse, nil
}

func (m mockCfn) DeleteStack(in *cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error) {
	return &m.DeleteStackResponse, nil
}

func TestMetadataUpdate(t *testing.T) {
	assert := assert.New(t)
	options := new(TestCases)
	options.GetTests("../../testcases/options.yaml")
	var bl *AwsBroker
	var bd *BucketDetailsRequest
	for _, v := range *options {
		bl, _ = NewAWSBroker(v, mockGetAwsSession, mockClients, mockGetAccountId, mockUpdateCatalog, mockPollUpdate)
		bd = &BucketDetailsRequest{
			v.S3Bucket,
			v.S3Key,
			v.TemplateFilter,
		}
	}
	bl.db.DataStorePort = mockDataStore{}

	s3svc := S3Client{
		Client: mockS3{GetObjectResp: s3.GetObjectOutput{}},
	}

	// test "__LISTINGS__" not in cache
	err := MetadataUpdate(bl.listingcache, bl.catalogcache, *bd, s3svc, bl.db, MetadataUpdate)
	assert.EqualError(err, "not found")

	// test empty s3 body
	var serviceUpdates []ServiceNeedsUpdate
	serviceUpdates = append(serviceUpdates, ServiceNeedsUpdate{
		Name:   "test-service",
		Update: true,
	})
	bl.listingcache.Set("__LISTINGS__", serviceUpdates)
	err = MetadataUpdate(bl.listingcache, bl.catalogcache, *bd, s3svc, bl.db, MetadataUpdate)
	assert.EqualError(err, "s3 object body missing")

	// test object not yaml
	s3obj := s3.GetObjectOutput{Body: ioutil.NopCloser(strings.NewReader("test"))}
	s3svc = S3Client{
		Client: mockS3{GetObjectResp: s3obj},
	}
	err = MetadataUpdate(bl.listingcache, bl.catalogcache, *bd, s3svc, bl.db, MetadataUpdate)
	assert.EqualError(err, "yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `test` into map[string]interface {}")

	// TODO: test success and more failure scenarios
}

func TestAssumeArnGeneration(t *testing.T) {
	params := map[string]string{"target_role_name": "worker"}
	accountId := "123456654321"
	assert.Equal(t, generateRoleArn(params, accountId), "arn:aws:iam::123456654321:role/worker", "Validate role arn")
	params["target_account_id"] = "000000000000"
	assert.Equal(t, generateRoleArn(params, accountId), "arn:aws:iam::000000000000:role/worker", "Validate role arn")
}