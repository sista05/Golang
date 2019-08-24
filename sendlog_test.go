package main

import (
	"bytes"
	"fmt"
	"testing"
	"context"
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"io/ioutil"
	"os"
)

func TestCompress(t *testing.T) {
	t.Run("compress", func(t *testing.T) {
		data := []Nginx{{"2019-08-23T15:37:26+09:00", "10.0.0.74", "13.112.30.41", "GET", "247", "/", "", "/index.php", "", "404", "323", "153", "-", "Mozilla/5.0 zgrab/0.x", "Root=1-5d36ab26-8a61c1cb8a4ae3503e77f20d", "-", "198.108.67.16", "0.000", "-"},
			{"2019-08-23T15:37:26+09:00", "10.0.0.74", "13.112.30.41", "GET", "247", "/", "", "/index.php", "", "404", "323", "153", "-", "Mozilla/5.0 zgrab/0.x", "Root=1-5d36ab26-8a61c1cb8a4ae3503e77f20d", "-", "198.108.67.16", "0.000", "-"}}

		datajson, _ := marshalAthena(data)
		var buf bytes.Buffer
		err := compress(&buf, datajson)
		if err != nil {
			t.Fatal("Error failed to compress")
		}
		if len(buf.Bytes()) == 0 {
			t.Fatal("Error failed to compress")
			t.Errorf("got: %v\nwant: %v", buf.Bytes(), 0)
		}
		fmt.Println("Test compress...")
	})
}

func TestWebhook(t *testing.T) {

	t.Run("webhook", func(t *testing.T) {
		err := webhook(true, "test13", "message")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("Test webhook...")
	})
}

func TestS3Upload(t *testing.T) {
	t.Run("upload", func(t *testing.T) {
		var buf bytes.Buffer
		data := []Nginx{{"2019-08-23T15:37:26+09:00", "10.0.0.74", "13.112.30.41", "GET", "247", "/", "", "/index.php", "", "404", "323", "153", "-", "Mozilla/5.0 zgrab/0.x", "Root=1-5d36ab26-8a61c1cb8a4ae3503e77f20d", "-", "198.108.67.16", "0.000", "-"},
			{"2019-08-23T15:37:26+09:00", "10.0.0.74", "13.112.30.41", "GET", "247", "/", "", "/index.php", "", "404", "323", "153", "-", "Mozilla/5.0 zgrab/0.x", "Root=1-5d36ab26-8a61c1cb8a4ae3503e77f20d", "-", "198.108.67.16", "0.000", "-"}}

		datajson, _ := marshalAthena(data)
		result, err := s3Upload(buf, datajson, "", "")
		if err != nil {
			t.Fatal("Error failed to s3upload ", err)
		}
		if result.Location == "" {
			t.Errorf("got: %v\nwant: %v", result.UploadID, "")
		}

		var appbuf bytes.Buffer

		raw, err := ioutil.ReadFile("./application.json")
		var app []Application
		json.Unmarshal(raw, &app)

		appjson, _ := marshalAthena(app)
		result, err = s3Upload(appbuf, appjson, "application", "application")
		if err != nil {
			t.Fatal("Error failed to s3upload")
		}
		if result.Location == "" {
			t.Errorf("got: %v\nwant: %v", result.UploadID, "")
		}

		fmt.Println("Test s3upload...")
	})
}

func TestHandler(t *testing.T) {
	t.Run("handler input test", func(t *testing.T) {
		raw, err := ioutil.ReadFile("./event_file.json")
		var event events.KinesisEvent
		json.Unmarshal(raw, &event)
		err = handler(context.Background(), event)
		if err != nil {
			t.Fatal("Error failed to kinesis event")
		}
		fmt.Println("Test handler...")
	})
}

func TestMain(m *testing.M) {
	println("before all...")

	os.Setenv("SLACK_WEBHOOK_URL", "https://hooks.slack.com/services/T02HDFTHD/BK560B15L/")
	os.Setenv("REGION", "ap-northeast-1")
	os.Setenv("S3_BUCKET", "sista05-development")

	code := m.Run()
	println("after all...")
	os.Exit(code)
}
