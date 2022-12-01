package main

import (
	"context"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

func main() {
	lambda.Start(setCWLogsRetention)
}

func setCWLogsRetention() {
	// create a map to store log group name and boolean pair
	// boolean will indicate whether the log group has retention policy set or not
	list := make(map[string]bool)

	// log group name to filter for
	patterns := []string{"/aws/lambda/*"}

	// check if environment variables are set
	var isEnvSet bool
	for _, v := range []string{"OMIT_LIST", "RETENTION_DAYS"} {
		_, isEnvSet = os.LookupEnv(v)
		if !isEnvSet {
			log.Fatalln("Missing required environment variable.")
		}
	}

	// get some necessary values from environment variables
	omitList := strings.Split(os.Getenv("OMIT_LIST"), ",") // comma separated list without whitespaces
	// valid values for "RETENTION_DAYS" are
	// 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 2192, 2556, 2922, 3288, 3653
	retentionDays, err := strconv.Atoi(os.Getenv("RETENTION_DAYS"))
	if err != nil {
		log.Fatal(err)
	}

	// initialize
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx) // load creds from credentials file or environment variables
	if err != nil {
		log.Fatal(err)
	}

	// initialize cloudwatchlogs service
	client := cloudwatchlogs.NewFromConfig(cfg)

	// create describe log groups inputs
	inputs := &cloudwatchlogs.DescribeLogGroupsInput{}

	// paginate the outputs using provided DescribeLogGroupsInput
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(client, inputs)
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			log.Fatal(err)
		}

		for _, v := range output.LogGroups {
			if slices.Contains(omitList, *v.LogGroupName) {
				continue
			}

			// filter based on provided patterns
			if matchPattern(patterns, *v.LogGroupName) {
				// add log groups to map
				// value is boolean
				// true = retention days are set
				// false = retention days are NOT set
				if v.RetentionInDays == nil {
					list[*v.LogGroupName] = false
				} else {
					list[*v.LogGroupName] = true
				}
			}
		}
	}

	for k, v := range list {
		// if log group does not have retention days set,
		// we will put retention days here
		if !v {
			policyInputs := &cloudwatchlogs.PutRetentionPolicyInput{
				LogGroupName:    aws.String(k),
				RetentionInDays: aws.Int32(int32(retentionDays)),
			}
			client.PutRetentionPolicy(ctx, policyInputs)
			log.Printf("%v\n", k)
		}
	}
}

func matchPattern(patterns []string, groupname string) bool {
	var m bool

	for _, p := range patterns {
		matched, err := regexp.MatchString(p, groupname)
		if err != nil {
			log.Fatal(err)
		}

		if matched {
			m = matched
			break // exit loop on first pattern match
		}
	}

	return m
}
