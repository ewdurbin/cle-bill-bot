package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/City-Bureau/chi-bill-bot/pkg/models"
	"github.com/City-Bureau/chi-bill-bot/pkg/svc"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

func SaveBillAndTweet(text string, bill *models.Bill, snsClient svc.SNSType) error {
	billJson, err := json.Marshal(bill)
	if err != nil {
		return err
	}
	err = snsClient.Publish(string(billJson), os.Getenv("SNS_TOPIC_ARN"), "save_bill")
	if err != nil {
		return err
	}
	data := svc.TweetData{Text: text, Params: twitter.StatusUpdateParams{InReplyToStatusID: *bill.TweetID}}
	tweetJson, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return snsClient.Publish(string(tweetJson), os.Getenv("SNS_TOPIC_ARN"), "post_tweet")
}

func HandleTweet(bill *models.Bill, db *gorm.DB, snsClient svc.SNSType) error {
	var billForTweet models.Bill

	if !db.Where(&models.Bill{TweetID: bill.TweetID}).Take(&billForTweet).RecordNotFound() {
		// Duplicate record already handled, exit
		return nil
	}

	if bill.BillID == "" {
		bill.Active = false
		return SaveBillAndTweet("Couldn't parse a bill identifier from the tweet", bill, snsClient)
	}

	var existingBill models.Bill
	if db.Where(&models.Bill{BillID: bill.BillID}).Take(&existingBill).RecordNotFound() {
		ocdBill := bill.GetOCDBill()
		if ocdBill.ID == "" {
			// Tweet that a valid bill wasn't found
			bill.Active = false
			return SaveBillAndTweet("Valid bill not found", bill, snsClient)
		}
		// Tweet that the new bill is now being tracked, save
		return SaveBillAndTweet(
			fmt.Sprintf("Bill now being tracked, you can follow with #%s", bill.BillID),
			bill,
			snsClient,
		)
	} else {
		// Tweet standard reply about already being able to follow it with hashtag
		existingBill.LastTweetID = bill.LastTweetID
		return SaveBillAndTweet(
			fmt.Sprintf("Bill already being tracked, you can follow with #%s", existingBill.BillID),
			&existingBill,
			snsClient,
		)
	}
}

func handler(request events.SNSEvent) error {
	if len(request.Records) < 0 {
		return nil
	}
	message := request.Records[0].SNS.Message
	db, err := gorm.Open("mysql", fmt.Sprintf(
		"%s:%s@tcp(%s:3306)/%s?parseTime=true",
		os.Getenv("RDS_USERNAME"),
		os.Getenv("RDS_PASSWORD"),
		os.Getenv("RDS_HOST"),
		os.Getenv("RDS_DB_NAME"),
	))
	snsClient := svc.NewSNSClient()
	if err != nil {
		// Log failure to trigger Lambda retry
		log.Fatal(err)
		return err
	}
	defer db.Close()

	var bill models.Bill
	err = json.Unmarshal([]byte(message), &bill)
	if err != nil {
		log.Fatal(err)
		return err
	}
	err = HandleTweet(&bill, db, snsClient)
	if err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
