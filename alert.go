package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/antonholmquist/jason"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"net/http"
	"os"
)

type Slack struct {
	WebhookURL string
	Channel    string
	Name       string
	Message    string
}

func send(s Slack) {
	jsonStr := `{"channel":"` + s.Channel +
		`","username":"` + s.Name +
		`","text":"` + s.Message + `"}`

	req, err := http.NewRequest(
		"POST",
		s.WebhookURL,
		bytes.NewBuffer([]byte(jsonStr)),
	)

	if err != nil {
		fmt.Print(err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Print(err)
	}

	fmt.Print(resp)
	defer resp.Body.Close()
}

func createMessage(message string) string {
	json, err := jason.NewObjectFromBytes([]byte(message))
	if err != nil {
		panic(err)
	}

	text := "<!channel>\n```\n"
	for k, v := range json.Map() {
		s, sErr := v.String()
		if sErr == nil {
			text += fmt.Sprintf("%s\t:%s\n", k, s)
		} else {
			text += fmt.Sprintf("%s\t:%s\n", k, sErr)
		}
	}
	text += "```"

	return text
}

// Send Slack notification from SNS event.
func slackNotice(ctx context.Context, snsEvent events.SNSEvent) {
	fmt.Printf("events %s \n", snsEvent.Records)

	for _, record := range snsEvent.Records {
		snsRecord := record.SNS
		fmt.Printf("[%s %s] Message = %s \n", record.EventSource, snsRecord.Timestamp, snsRecord.Message)

		var s = Slack{os.Getenv("SLACK_WEBHOOK_URL"),
			os.Getenv("SLACK_CHANNEL"),
			os.Getenv("SLACK_NAME"),
			createMessage(snsRecord.Message)}

		send(s)
	}
}

func main() {
	lambda.Start(slackNotice)
}
