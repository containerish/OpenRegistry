package dfs

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyendpoints "github.com/aws/smithy-go/endpoints"
)

type EndpointResolver struct {
	endpoint *string
}

func (er *EndpointResolver) ResolveEndpoint(
	ctx context.Context,
	params s3.EndpointParameters,
) (smithyendpoints.Endpoint, error) {
	params.Region = aws.String("us-east-1")
	params.Endpoint = er.endpoint

	return s3.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, params)
}

func NewS3Client(endpoint, accessKey, secretKey string) *s3.Client {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		log.Fatalf("ERR_LOADING_DEFAULT_CREDS: %s", err)
		return nil
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.EndpointResolverV2 = &EndpointResolver{
			endpoint: aws.String(endpoint),
		}
	})
	return client
}
