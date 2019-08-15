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
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"github.com/sha1sum/aws_signing_client"
	"gopkg.in/olivere/elastic.v6"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Nginx struct {
	Time                   string `json:"time"`
	Remote_addr            string `json:"remote_addr"`
	Host                   string `json:"host"`
	Request_method         string `json:"request_method"`
	Request_length         string `json:"request_length"`
	Request_uri            string `json:"request_uri"`
	Https                  string `json:"https"`
	Uri                    string `json:"uri"`
	Query_string           string `json:"query_string"`
	Status                 string `json:"status"`
	Bytes_sent             string `json:"bytes_sent"`
	Body_bytes_sent        string `json:"body_bytes_sent"`
	Referer                string `json:"referer"`
	Useragent              string `json:"useragent"`
	Amzn_trace_id          string `json:"http_x_amzn_trace_id"`
	Amzn_agw_api_id        string `json:"http_x_amzn_apigateway_api_id"`
	Forwardedfor           string `json:"forwardedfor"`
	Request_time           string `json:"request_time"`
	Upstream_response_time string `json:"upstream_response_time"`
}

type Application struct {
	Id         string   `json:"id"`
	System     string   `json:"system"`
	Level      string   `json:"level"`
	Datetime   string   `json:"datetime"`
	Env        string   `json:"env"`
	Message    string   `json:"message"`
	Code       string   `json:"code"`
	Response   string   `json:"response"`
	Trace      []string `json:"trace"`
	Genre      string   `json:"genre"`
	Parameters string   `json:"parameters"`
	Slack      struct {
		Notification bool `json:"notification"`
		Body         struct {
			SendChannel string `json:"send_channel"`
			AtChannel   bool   `json:"at_channel"`
			Message     string `json:"Message"`
			Id          string `json:"Id"`
			Level       string `json:"Level"`
		} `json:"body"`
	} `json:"slack"`
	Extra struct {
		File       string `json:"file"`
		Line       string `json:"line"`
		Class      string `json:"class"`
		Function   string `json:"function"`
		ProcessIdi string `json:"process_id"`
		URL        string `json:"url"`
		IP         string `json:"ip"`
		HttpMethod string `json:"http_method"`
		Server     string `json:"server"`
		Referrer   string `json:"referrer"`
	} `json:"extra"`
}

type Nginxs []Nginx
type Applications []Application

// send notification to slack.
func webhook(at_channel bool, channel string, message string) error {

	if at_channel {
		message = "<!channel> " + message
	}

	jsonStr := struct {
		Channel  string `json:"channel"`
		Username string `json:"username"`
		Text     string `json:"text"`
	}{
		Channel:  channel,
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
	fmt.Println("send message", string(b))
	if err != nil {
		return errors.Wrap(err, "Error failed to send Slack")
	}
	defer resp.Body.Close()

	return err
}

func elasticClient() (*elastic.Client, error) {

	creds := credentials.NewEnvCredentials()
	signer := v4.NewSigner(creds)

	awsClient, err := aws_signing_client.New(signer, nil, "es", os.Getenv("REGION"))
	if err != nil {
		return nil, err
	}
	return elastic.NewClient(
		elastic.SetURL(os.Getenv("ES_URL")),
		elastic.SetScheme("https"),
		elastic.SetHttpClient(awsClient),
		elastic.SetSniff(false),
	)
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

// send logdata(json) to s3
func s3Upload(buf bytes.Buffer, convertData []byte, logname string, hostname string) (*s3manager.UploadOutput, error) {

	err := compress(&buf, []byte(convertData))
	if err != nil {
		return nil, errors.Wrap(err, "Error failed compress")
	}

	sess := createSession()

	var uploader = s3manager.NewUploader(sess)

	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	t := time.Now().In(jst).Format("2006/01/02 15:04:05")
	tmp := strings.FieldsFunc(t, split)

	// if argument has hostname, function gives hostname to logname.
	if hostname != "" {
		hostname = hostname + "-"
	}
	path := "/" + logname + "/" + strings.Join(tmp[0:4], "/") + "/" + hostname + strings.Join(tmp, "") + "-" + logname + ".gz"

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

//For JSON lines (support athena JSON SerDe libraries)
func marshalAthena(v interface{}) ([]byte, error) {
	vathena, err := json.Marshal(v)
	vathena = bytes.ReplaceAll(vathena, []byte("[{"), []byte("{"))
	vathena = bytes.ReplaceAll(vathena, []byte("},{"), []byte("},\n{"))
	vathena = bytes.ReplaceAll(vathena, []byte("}]"), []byte("}"))
	return vathena, err
}

// remove duplicate hostname
func removeDuplicate(hostname []string) []string {
	results := make([]string, 0, len(hostname))
	encountered := map[string]bool{}
	for i := 0; i < len(hostname); i++ {
		if !encountered[hostname[i]] {
			encountered[hostname[i]] = true
			results = append(results, hostname[i])
		}
	}
	return results
}

// Filter returns a new slice containig hostname.
func nginxsFilter(vs Nginxs, f func(string) bool) Nginxs {
	vsf := make(Nginxs, 0)
	for _, v := range vs {
		if f(v.Host) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func handler(ctx context.Context, kinesisEvent events.KinesisEvent) error {

	var dataBytes []byte
	var nginx Nginx
	var nginxs Nginxs
	var nginxtmp Nginxs
	var application Application
	var applications Applications

	for _, record := range kinesisEvent.Records {
		kinesisRecord := record.Kinesis
		dataBytes = kinesisRecord.Data

		// Extract substring from KinesisRecord
		if bytes.Contains(dataBytes, []byte("forwardedfor")) == true {
			json.Unmarshal(dataBytes, &nginx)
			nginxs = append(nginxs, nginx)
		} else if bytes.Contains(dataBytes, []byte("extra")) == true {
			json.Unmarshal(dataBytes, &application)
			fmt.Println(application)
			applications = append(applications, application)
		}
	}

	// nginx log processing
	if nginxs != nil {
		var nginxbuf bytes.Buffer
		var hostnameList []string
		var hostnameUniqueList []string

		for _, tmp := range nginxs {
			hostnameList = append(hostnameList, tmp.Host)
		}

		hostnameUniqueList = removeDuplicate(hostnameList)

		// sort access log by hostname.
		for _, tmp := range hostnameUniqueList {
			nginxtmp = nginxsFilter(nginxs, func(v string) bool {
				return v == tmp
			})
			nginxsjson, _ := marshalAthena(nginxtmp)
			_, err := s3Upload(nginxbuf, nginxsjson, "nginx_access", tmp)
			if err != nil {
				return errors.Wrap(err, "Error failed to s3 upload")
			}
		}

		cli, err := elasticClient()
		if err != nil {
			return errors.Wrap(err, "Error failed to elasticsearch access")
		}

		// for elasticsearch data structure
		for _, tmp := range nginxs {
			accessdata := Nginx{
				Time:                   tmp.Time,
				Remote_addr:            tmp.Remote_addr,
				Host:                   tmp.Host,
				Request_method:         tmp.Request_method,
				Request_length:         tmp.Request_length,
				Request_uri:            tmp.Request_uri,
				Https:                  tmp.Https,
				Uri:                    tmp.Uri,
				Query_string:           tmp.Query_string,
				Status:                 tmp.Status,
				Bytes_sent:             tmp.Bytes_sent,
				Body_bytes_sent:        tmp.Body_bytes_sent,
				Referer:                tmp.Referer,
				Useragent:              tmp.Useragent,
				Amzn_trace_id:          tmp.Amzn_trace_id,
				Amzn_agw_api_id:        tmp.Amzn_agw_api_id,
				Forwardedfor:           tmp.Forwardedfor,
				Request_time:           tmp.Request_time,
				Upstream_response_time: tmp.Upstream_response_time,
			}

			_, err = cli.Index().Index(os.Getenv("ES_NGINX_INDEX")).Type(os.Getenv("ES_NGINX_INDEXTYPE")).
				BodyJson(accessdata).
				Do(ctx)
			if err != nil {
				return errors.Wrap(err, "Error failed to elasticsearch PUT")
			}
		}


	  // laravel log processing
		if applications != nil {
			var applicationbuf bytes.Buffer

			for _, record := range applications {

				// Flag on, send notify.
				if record.Slack.Notification {
					err := webhook(record.Slack.Body.AtChannel, record.Slack.Body.SendChannel, record.Slack.Body.Message)
					if err != nil {
						return errors.Wrap(err, "Error failed to send application notification to slack")
					}
				}
			}

			applicationsjson, _ := marshalAthena(applications)
			_, err := s3Upload(applicationbuf, applicationsjson, "application", "")
			if err != nil {
				return errors.Wrap(err, "Error failed to s3 upload")
			}

			cli, err := elasticClient()
			if err != nil {
				return errors.Wrap(err, "Error failed to elasticsearch access")
			}

			// for elasticsearch data structure
			for _, k := range applications {
				k.Datetime = k.Datetime + "+09:00" // adjust elasticsearch timezone
				esdata := Application{
					Id:         k.Id,
					System:     k.System,
					Level:      k.Level,
					Datetime:   k.Datetime,
					Env:        k.Env,
					Message:    k.Message,
					Code:       k.Code,
					Response:   k.Response,
					Trace:      k.Trace,
					Genre:      k.Genre,
					Parameters: k.Parameters,
					Slack:      k.Slack,
					Extra:      k.Extra,
				}

				// laravel logs datetime types is unmatched Elasticsearch dynamic mappings.
				mapping := fmt.Sprintf(`{
					"mappings": {
						"%s": {
							"properties": {
								"datetime": {
									"type": "date",
									"format": "yyyy-MM-dd HH:mm:ssZ"
								}
							}
						}
					}
				}`, os.Getenv("ES_APP_INDEXTYPE"))

				// In case creating Elasticsearch index, define mapping first.
				// (dynamic mapping not working)
				exists, err := cli.IndexExists(os.Getenv("ES_APP_INDEX")).Do(ctx)
				if !exists {
					_, err = cli.CreateIndex(os.Getenv("ES_APP_INDEX")).BodyString(mapping).Do(ctx)
					if err != nil {
						return errors.Wrap(err, "Error failed to elasticsearch PUT")
					}
				}

				_, err = cli.Index().Index(os.Getenv("ES_APP_INDEX")).Type(os.Getenv("ES_APP_INDEXTYPE")).
					BodyJson(esdata).
					Do(ctx)
				if err != nil {
					return errors.Wrap(err, "Error failed to elasticsearch PUT")
				}
			}
		}
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
