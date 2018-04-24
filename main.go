package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// https://stackoverflow.com/questions/25254443/return-local-beginning-of-day-time-object-in-go/25254593#25254593
func Bod(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func parse_d_tag(tag string) (t time.Time, err error) {
	const shortForm = "d_2006_01_02"

	t, err = time.Parse(shortForm, tag)
	return t, err
}

func run() error {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		return err
	}
	s3svc := s3.New(sess)

	inputparams := &s3.ListObjectsInput{
		Bucket:  aws.String("eups.lsst.codes"),
		MaxKeys: aws.Int64(500),
	}

	re := regexp.MustCompile(`d_\d{4}_\d{2}_\d{2}\.list$`)

	var objs []string

	pageNum := 0
	err = s3svc.ListObjectsPages(inputparams, func(page *s3.ListObjectsOutput, lastPage bool) bool {
		fmt.Println("Page", pageNum)
		pageNum++
		for _, value := range page.Contents {
			if re.MatchString(*value.Key) {
				objs = append(objs, *value.Key)
				fmt.Println(*value.Key)
			}
		}
		fmt.Println("pageNum", pageNum, "lastPage", lastPage)

		// return if we should continue with the next page
		//return true
		return false
	})
	if err != nil {
		return err
	}

	pacific, err := time.LoadLocation("US/Pacific")
	if err != nil {
		return err
	}
	today := Bod(time.Now().In(pacific))
	fmt.Println(today)
	var old_tag_dir = "old_tags"
	type keyvalue map[string]interface{}
	var stale_objs []keyvalue
	for _, k := range objs {
		fmt.Println("key: ", k)

		dir, file := filepath.Split(k)
		base := filepath.Base(dir)

		fmt.Println("base: ", base)
		fmt.Println("file: ", file)

		if base != old_tag_dir {
			target := path.Join(dir, old_tag_dir, file)
			d_tag, err := parse_d_tag(strings.TrimSuffix(file, ".list"))
			if err != nil {
				return err
			}
			m := keyvalue{
				"current_key": k,
				"target_key":  target,
				"time":        d_tag,
			}
			stale_objs = append(stale_objs, m)
		}
	}

	for _, k := range stale_objs {
		fmt.Println(k)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
