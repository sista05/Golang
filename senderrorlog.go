package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type NginxError struct {
	Logname   string `json:"nginx_error"`
	Timestamp string `json:"time_stamp"`
	Loglevel  string `json:"log_level"`
	Message   string `json:"message"`
}

type PhpError struct {
	Logname   string `json:"php-fpm-error"`
	Timestamp string `json:"time_stamp"`
	Loglevel  string `json:"log_level"`
	Message   string `json:"message"`
}

type NginxErrors []NginxError
type PhpErrors []PhpError

// send notification to slack.
func webhook(message string) error {

	jsonStr := struct {
		Channel  string `json:"channel"`
		Username string `json:"username"`
		Text     string `json:"text"`
	}{
		Channel:  os.Getenv("SLACK_CHANNEL"),
		Username: os.Getenv("SLACK_NAME"),
		Text:     message,
	}

	b, _ := json.Marshal(jsonStr)
	req, err := http.NewRequest(
		"POST",
		os.Getenv("SLACK_WEBHOOK_URL"),
		bytes.NewBuffer(b),
	)
	if err != nil {
		return errors.Wrap(err, "Error failed to create message")
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Error failed to send Slack")
	}
	fmt.Println("send message", string(b))
	defer resp.Body.Close()

	return err
}

func createSession() *session.Session {

	var sess = session.Must(session.NewSession(&aws.Config{
		S3ForcePathStyle: aws.Bool(true),
		Region:           aws.String(os.Getenv("REGION")),
		Endpoint:         aws.String(os.Getenv("S3_ENDPOINT")),
	}))
	return sess
}

// split nowtime to separate strings by space corone slash.
func split(r rune) bool {
	return r == ':' || r == ' ' || r == '/'
}

func s3Upload(buf bytes.Buffer, convertData []byte, logname string) (*s3manager.UploadOutput, error) {

	err := compress(&buf, []byte(convertData))
	if err != nil {
		return nil, errors.Wrap(err, "Error failed compress")
	}

	sess := createSession()

	var uploader = s3manager.NewUploader(sess)

	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	t := time.Now().In(jst).Format("2006/01/02 15:04:05")
	tmp := strings.FieldsFunc(t, split)

	path := "/" + logname + "/" + strings.Join(tmp[0:4], "/") + "/" + strings.Join(tmp, "") + "-" + logname + ".gz"

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(path),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to upload file")
	}

	return result, err
}

// gunzip logdata
func compress(w io.Writer, convertData []byte) error {
	gw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	gw.Write(convertData)
	defer gw.Close()
	return err
}

//For JSON lines (support athena JSON SerDe libraries)func marshalAthena(v interface{}) ([]byte, error) {
func marshalAthena(v interface{}) ([]byte, error) {
	vathena, err := json.Marshal(v)
	vathena = bytes.ReplaceAll(vathena, []byte("[{"), []byte("{"))
	vathena = bytes.ReplaceAll(vathena, []byte("},{"), []byte("},\n{"))
	vathena = bytes.ReplaceAll(vathena, []byte("}]"), []byte("}"))
	return vathena, err
}

func handler(ctx context.Context, kinesisEvent events.KinesisEvent) error {

	var dataBytes []byte
	var nginxerror NginxError
	var nginxerrors NginxErrors
	var phperror PhpError
	var phperrors PhpErrors

	for _, record := range kinesisEvent.Records {
		kinesisRecord := record.Kinesis
		dataBytes = kinesisRecord.Data

		// Extract substring from KinesisRecord
		if bytes.Contains(dataBytes, []byte("nginx_error")) != false {
			json.Unmarshal(dataBytes, &nginxerror)
			nginxerrors = append(nginxerrors, nginxerror)
		} else if bytes.Contains(dataBytes, []byte("php-fpm-error")) == true {
			json.Unmarshal(dataBytes, &phperror)
			t, _ := time.Parse("02-Jan-2006 15:04:05", phperror.Timestamp)
			phperror.Timestamp = t.Format("2006/01/02 15:04:05") // for athena format
			phperrors = append(phperrors, phperror)
		}
	}

	if nginxerrors != nil {
		var nginxerrorbuf bytes.Buffer

		// When loglevel is error, send a slack notification.
		for _, record := range nginxerrors {
			if record.Loglevel == "error" {
				err := webhook(record.Message)
				if err != nil {
					return errors.Wrap(err, "Error failed to send nginx_error notification to slack")
				}
			}
		}

		nginxerrorjson, _ := marshalAthena(nginxerrors)
		_, err := s3Upload(nginxerrorbuf, nginxerrorjson, "nginx_error")
		if err != nil {
			return errors.Wrap(err, "Error failed to s3 upload")
		}
	}

	if phperrors != nil {
		var phperrorbuf bytes.Buffer

		// When loglevel is higher than warning, send a slack notification.
		for _, record := range phperrors {
			if record.Loglevel != "NOTICE" {
				err := webhook(record.Message)
				if err != nil {
					return errors.Wrap(err, "Error failed to send php-fpm-error notification to slack")
				}
			}
		}

		phperrorjson, _ := marshalAthena(phperrors)
		_, err := s3Upload(phperrorbuf, phperrorjson, "php-fpm-error")
		if err != nil {
			return errors.Wrap(err, "Error failed to s3 upload")
		}
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
