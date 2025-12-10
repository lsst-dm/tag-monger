package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/jessevdk/go-flags"
	"google.golang.org/api/iterator"
)

type Tags struct {
	key        string
	base       string
	file       string
	time       time.Time
	target_key string
	tag        string
}

var opts struct {
	Verbose    bool   `short:"v" long:"verbose" description:"Show verbose debug information" env:"TAG_MONGER_VERBOSE"`
	PageSize   int64  `short:"p" long:"pagesize" description:"page size of s3 object listing" default:"100" env:"TAG_MONGER_PAGESIZE"`
	MaxObjects int64  `short:"m" long:"max" description:"maximum number of s3 object to list" default:"1000" env:"TAG_MONGER_MAX"`
	Bucket     string `short:"b" long:"bucket" description:"name of s3 bucket" required:"true" env:"TAG_MONGER_BUCKET"`
	Days       int    `short:"d" long:"days" description:"Expire tags older than N days" default:"14" env:"TAG_MONGER_DAYS"`
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

func gcs_fetch_objects(ctx context.Context, client storage.Client, bucket_name string) ([]string, error) {
	fmt.Println("looking for objects in bucket:", bucket_name)

	var objs []string
	it := client.Bucket(bucket_name).Objects(ctx, nil)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate objects: %v", err)
		}

		objs = append(objs, attrs.Name)

		if opts.MaxObjects > 0 && int64(len(objs)) >= opts.MaxObjects {
			fmt.Printf("found %d objects\n", len(objs))
			break
		}
		if len(objs)%10000 == 0 {
			fmt.Printf("Loaded %d files from bucket\n", len(objs))
		}
	}

	return objs, nil
}

func filter_objects(objs []string, pattern string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	fmt.Println("looking for tags like:", pattern)
	var match_objs []string
	for _, k := range objs {
		if re.MatchString(k) {
			if opts.Verbose {
				fmt.Println(k)
			}
			match_objs = append(match_objs, k)
		}
	}

	return match_objs, nil
}

func parse_objects(objs []string, tz string, max_days int) ([]Tags, []Tags, []Tags, error) {
	pacific, err := time.LoadLocation(tz)
	if err != nil {
		return nil, nil, nil, err
	}

	today := bod(time.Now().In(pacific))
	fmt.Println("today:", today)
	cutoff_date := today.AddDate(0, 0, -(max_days - 1))
	fmt.Println("expire tags prior to", cutoff_date)

	fmt.Println("groking objects...")

	old_tag_dir := "old_tags"
	var fresh_objs []Tags
	var expired_objs []Tags
	var retired_objs []Tags
	for _, k := range objs {
		dir, file := filepath.Split(k)
		base := filepath.Base(dir)

		p := Tags{
			key:  k,
			file: file,
			base: base,
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
			fmt.Println("Error parsing tag name:", tag_name)
			continue
		}

		p.time = tag_date
		p.tag = tag_name

		if !tag_date.Before(cutoff_date) {
			fresh_objs = append(fresh_objs, p)
			continue
		}

		target := path.Join(dir, old_tag_dir, file)
		p.target_key = target

		expired_objs = append(expired_objs, p)
	}
	fmt.Printf("found %d \"fresh enough\" eups tag files\n", len(fresh_objs))
	fmt.Printf("found %d expired eups tag files\n", len(expired_objs))
	fmt.Printf("found %d retired eups tag files\n", len(retired_objs))

	return fresh_objs, expired_objs, retired_objs, nil
}

func gcs_mv_object(ctx context.Context, client *storage.Client, src_bkt string, src_key string, dst_bkt string, dst_key string) error {
	srcObj := client.Bucket(src_bkt).Object(src_key)
	dstObj := client.Bucket(dst_bkt).Object(dst_key)

	// copy obj
	fmt.Printf("Copying %s to %s", src_bkt+src_key, dst_bkt+dst_key)
	_, err := dstObj.CopierFrom(srcObj).Run(ctx)
	if err != nil {
		return err
	}
	// delete obj
	err = srcObj.Delete(ctx)
	if err != nil {
		return err
	}
	return nil
}

/*
 * Note: This program is intentionally serialized as this makes it easier to
 * develop the workflow.  It should be easy to convert to concurrent s3
 * operations if needed for performance in the future.
 */
func run() error {
	// The default behavior of flags.Parse() includes flags.HelpFlag, which
	// results in the usage message being printed twice if we are manually
	// printing the usage message on any flag error. Thus, -h|--help is being
	// own way.
	p := flags.NewParser(&opts, flags.PassDoubleDash)
	_, err := p.Parse()
	if err != nil {
		fmt.Print(err)
		fmt.Println()

		var b bytes.Buffer
		p.WriteHelp(&b)
		return errors.New(b.String())
	}
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	objs, err := gcs_fetch_objects(ctx, *client, opts.Bucket)
	if err != nil {
		return err
	}
	println("Found", len(objs), "total items")

	taglike_objs, err := filter_objects(objs, `d_\d{4}_\d{2}_\d{2}\.list$`)
	if err != nil {
		return err
	}
	println("Found", len(taglike_objs), "total tags")

	fresh_objs, expired_objs, retired_objs, err := parse_objects(
		taglike_objs, "America/Los_Angeles", opts.Days)
	if err != nil {
		return err
	}
	process_tags(retired_objs, fresh_objs, expired_objs, client, ctx)
	return nil
}

func process_tags(retired_objs []Tags, fresh_objs []Tags, expired_objs []Tags, svc any, ctx context.Context) error {
	if opts.Verbose {
		fmt.Println("already retried objects:")
		for _, k := range retired_objs {
			fmt.Println(k.key)
		}

		fmt.Println("\"fresh enough\" objects:")
		for _, k := range fresh_objs {
			fmt.Println(k.key)
		}

		fmt.Println("expired objects:")
		for _, k := range expired_objs {
			fmt.Println(k.key)
		}
	}

	for _, k := range expired_objs {
		if opts.Verbose {
			fmt.Println("renaming", k.key)
			fmt.Println("    -> ", k.target_key)
		}

		if !opts.Noop {
			err := gcs_mv_object(
				ctx,
				svc.(*storage.Client),
				opts.Bucket,
				k.key,
				opts.Bucket,
				k.target_key)
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
