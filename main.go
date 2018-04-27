package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jessevdk/go-flags"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type keyvalue map[string]interface{}

var opts struct {
	Verbose    bool   `short:"v" long:"verbose" description:"Show verbose debug information" env:"TAG_MONGER_VERBOSE"`
	PageSize   int64  `short:"p" long:"pagesize" description:"page size of s3 object listing" default:"100" env:"TAG_MONGER_PAGESIZE"`
	MaxObjects int64  `short:"m" long:"max" description:"maximum number of s3 object to list" default:"1000" env:"TAG_MONGER_MAX"`
	Bucket     string `short:"b" long:"bucket" description:"name of s3 bucket" required:"true" env:"TAG_MONGER_BUCKET"`
	Days       int    `short:"d" long:"days" description:"Expire tags older than N days" default:"30" env:"TAG_MONGER_DAYS"`
	Noop       bool   `short:"n" long:"noop" description:"Do not make any changes" env:"TAG_MONGER_NOOP"`
	Group      struct {
		Help bool `short:"h" long:"help" description:"Show this help message"`
	} `group:"Help Options"`
}

// https://stackoverflow.com/questions/25254443/return-local-beginning-of-day-time-object-in-go/25254593#25254593
func bod(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func parse_d_tag(tag string) (t time.Time, err error) {
	const shortForm = "d_2006_01_02"

	t, err = time.Parse(shortForm, tag)
	return t, err
}

func fetch_objects(s3svc *s3.S3, bucket_name string, page_size int64) ([]string, error) {
	inputparams := &s3.ListObjectsInput{
		Bucket:  aws.String(bucket_name),
		MaxKeys: aws.Int64(page_size),
	}

	fmt.Println("looking for objects in bucket:", *inputparams.Bucket)
	fmt.Println("page size:", *inputparams.MaxKeys)

	var objs []string
	pageNum := 0
	err := s3svc.ListObjectsPages(
		inputparams,
		func(page *s3.ListObjectsOutput, lastPage bool) bool {
			if opts.Verbose {
				fmt.Println("Page", pageNum)
			}
			pageNum++
			for _, value := range page.Contents {
				objs = append(objs, *value.Key)
			}
			if opts.MaxObjects > 0 && int64(len(objs)) >= opts.MaxObjects {
				return false
			}

			// return if we should continue with the next page
			return true
		})
	if err != nil {
		return nil, err
	}

	fmt.Printf("found %d objects\n", len(objs))

	return objs, nil
}

func filter_objects(objs []string, pattern string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	fmt.Println("looking for objects like:", pattern)
	var match_objs []string
	for _, k := range objs {
		if re.MatchString(k) {
			if opts.Verbose {
				fmt.Println(k)
			}
			match_objs = append(match_objs, k)
		}
	}
	fmt.Printf("found %d objects\n", len(match_objs))

	return match_objs, nil
}

func parse_objects(objs []string, tz string, max_days int) ([]keyvalue, []keyvalue, []keyvalue, error) {
	pacific, err := time.LoadLocation(tz)
	if err != nil {
		return nil, nil, nil, err
	}

	today := bod(time.Now().In(pacific))
	fmt.Println("today:", today)
	cutoff_date := today.AddDate(0, 0, (max_days - 1))
	fmt.Println("expire tags prior to", cutoff_date)

	fmt.Println("groking objects...")

	var old_tag_dir = "old_tags"
	var fresh_objs []keyvalue
	var expired_objs []keyvalue
	var retired_objs []keyvalue
	for _, k := range objs {
		dir, file := filepath.Split(k)
		base := filepath.Base(dir)

		p := keyvalue{
			"key":  k,
			"file": file,
			"base": base,
		}

		if base == old_tag_dir {
			// already retried
			retired_objs = append(retired_objs, p)
			continue
		}

		// do not bother to further parse retired tags
		tag_name := strings.TrimSuffix(file, ".list")
		tag_date, err := parse_d_tag(tag_name)
		if err != nil {
			return nil, nil, nil, err
		}

		p["time"] = tag_date
		p["tag"] = tag_name

		if !tag_date.Before(cutoff_date) {
			fresh_objs = append(fresh_objs, p)
			continue
		}

		target := path.Join(dir, old_tag_dir, file)
		p["target_key"] = target

		expired_objs = append(expired_objs, p)
	}
	fmt.Printf("found %d \"fresh enough\" eups tag files\n", len(fresh_objs))
	fmt.Printf("found %d expired eups tag files\n", len(expired_objs))
	fmt.Printf("found %d retired eups tag files\n", len(retired_objs))

	return fresh_objs, expired_objs, retired_objs, nil
}

func mv_object(s3svc *s3.S3, src_bkt string, src_key string, dst_bkt string, dst_key string) error {
	// copy object
	copyinput := &s3.CopyObjectInput{
		CopySource: aws.String(src_bkt + "/" + src_key),
		Bucket:     aws.String(dst_bkt),
		Key:        aws.String(dst_key),
	}
	_, err := s3svc.CopyObject(copyinput)
	if err != nil {
		return err
	}

	// wait for new object
	dst_headinput := &s3.HeadObjectInput{
		Bucket: aws.String(dst_bkt),
		Key:    aws.String(dst_key),
	}
	err = s3svc.WaitUntilObjectExists(dst_headinput)
	if err != nil {
		return err
	}

	// delete source object
	deleteinput := &s3.DeleteObjectInput{
		Bucket: aws.String(src_bkt),
		Key:    aws.String(src_key),
	}
	_, err = s3svc.DeleteObject(deleteinput)
	if err != nil {
		return err
	}

	// wait for old object to die
	src_headinput := &s3.HeadObjectInput{
		Bucket: aws.String(src_bkt),
		Key:    aws.String(src_key),
	}
	err = s3svc.WaitUntilObjectNotExists(src_headinput)
	if err != nil {
		return err
	}

	return nil
}

/*
 * Note: This program is intentional serialized as this makes it easier to
 * develope the workflow.  It should be easy to convert to concurrent s3
 * operations if needed for performance in the future.
 */
func run() error {
	// the default behavior of flags.Parse() includes flags.HelpFlag, which
	// results in the usage message being printed twice if we are manually
	// printing the usage message on any flag error. Thus, -h|--help is being
	// own way.
	p := flags.NewParser(&opts, flags.PassDoubleDash)
	_, err := p.Parse()
	if err != nil {
		fmt.Println(err, "\n")

		var b bytes.Buffer
		p.WriteHelp(&b)
		return errors.New(b.String())
	}

	sess, err := session.NewSession(&aws.Config{
		// $AWS_REGION must be set if this is ommited.
		//Region: aws.String("us-east-1"),
		CredentialsChainVerboseErrors: aws.Bool(true),
	})
	if err != nil {
		return err
	}
	s3svc := s3.New(sess)

	// it would be more memory efficent to loop over objects as they are
	// fetched and this might be required for buckets with a large number of
	// objects. However, it is slightly easier to refactor/recompose as a
	// pipeline of several small steps.
	objs, err := fetch_objects(s3svc, opts.Bucket, opts.PageSize)
	if err != nil {
		return err
	}

	taglike_objs, err := filter_objects(objs, `d_\d{4}_\d{2}_\d{2}\.list$`)
	if err != nil {
		return err
	}

	fresh_objs, expired_objs, retired_objs, err := parse_objects(
		taglike_objs, "US/Pacific", opts.Days)
	if err != nil {
		return err
	}

	if opts.Verbose {
		fmt.Println("already retried objects:")
		for _, k := range retired_objs {
			fmt.Println(k["key"])
		}

		fmt.Println("\"fresh enough\" objects:")
		for _, k := range fresh_objs {
			fmt.Println(k["key"])
		}

		fmt.Println("expired objects:")
		for _, k := range expired_objs {
			fmt.Println(k["key"])
		}
	}

	for _, k := range expired_objs {
		if opts.Verbose {
			fmt.Println("renaming", k["key"])
			fmt.Println("    -> ", k["target_key"])
		}

		if !opts.Noop {
			err := mv_object(
				s3svc,
				opts.Bucket,
				k["key"].(string),
				opts.Bucket,
				k["target_key"].(string))
			if err != nil {
				return err
			}
		} else {
			fmt.Println("    (noop)")
		}
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
