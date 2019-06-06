package main

import (
	"encoding/json"

	"github.com/City-Bureau/chi-bill-bot/pkg/svc"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

// Event should include bill and params
func handler(request events.SNSEvent) error {
	if len(request.Records) < 0 {
		return nil
	}
	message := request.Records[0].SNS.Message

	var data svc.TweetData
	err := json.Unmarshal([]byte(message), data)
	if err != nil {
		return err
	}

	twttr := svc.NewTwitterClient()
	twttr.PostTweet(data.Text, &data.Params)
	return nil
}

func main() {
	lambda.Start(handler)
}
