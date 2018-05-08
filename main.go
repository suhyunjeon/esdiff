package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/olivere/esdiff/diff"
	"github.com/olivere/esdiff/diff/printer"
	"github.com/olivere/esdiff/elastic"
	"github.com/olivere/esdiff/elastic/config"
	"github.com/olivere/esdiff/elastic/v5"
	"github.com/olivere/esdiff/elastic/v6"
)

func main() {
	var (
		outputFormat = flag.String("o", "", "Output format, e.g. json")
		sort         = flag.String("sort", "", "Sort field, e.g. _id or name.keyword")
		size         = flag.Int("size", 100, "Batch size")
		rawSrcFilter = flag.String("sf", "", `Raw query for filtering the source, e.g. {"term":{"user":"olivere"}}`)
		rawDstFilter = flag.String("df", "", `Raw query for filtering the destination, e.g. {"term":{"name.keyword":"Oliver"}}`)
	)
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 2 {
		usage()
		os.Exit(1)
	}

	options := []elastic.ClientOption{
		elastic.WithSortField(*sort),
		elastic.WithBatchSize(*size),
	}

	src, err := newClient(flag.Arg(0), options...)
	if err != nil {
		log.Fatal(err)
	}
	srcIterReq := &elastic.IterateRequest{
		RawQuery: *rawSrcFilter,
	}

	dst, err := newClient(flag.Arg(1), options...)
	if err != nil {
		log.Fatal(err)
	}
	dstIterReq := &elastic.IterateRequest{
		RawQuery: *rawDstFilter,
	}

	var p printer.Printer
	{
		switch *outputFormat {
		default:
			p = printer.NewStdPrinter(os.Stdout)
		case "json":
			p = printer.NewJSONPrinter(os.Stdout, -1, 0)
		}
	}

	g, ctx := errgroup.WithContext(context.Background())
	srcDocCh, srcErrCh := src.Iterate(ctx, srcIterReq)
	dstDocCh, dstErrCh := dst.Iterate(ctx, dstIterReq)
	doneCh := make(chan struct{}, 1)
	diffCh, errCh := diff.Differ(ctx, srcDocCh, dstDocCh)
	g.Go(func() error {
		defer close(doneCh)
		for {
			select {
			case d, ok := <-diffCh:
				if !ok {
					return nil
				}
				p.Print(d)
			case err := <-srcErrCh:
				return err
			case err := <-dstErrCh:
				return err
			case err := <-errCh:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	if err = g.Wait(); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "General usage:\n\n")
	fmt.Fprintf(os.Stderr, "\t%s [flags] <source-url> <destination-url>\n\n", path.Base(os.Args[0]))
	fmt.Fprintf(os.Stderr, "General flags:\n")
	flag.PrintDefaults()
}

// newClient will create a new Elasticsearch client,
// matching the supported version.
func newClient(url string, opts ...elastic.ClientOption) (elastic.Client, error) {
	cfg, err := config.Parse(url)
	if err != nil {
		return nil, err
	}
	v, major, _, _, err := elasticsearchVersion(cfg)
	if err != nil {
		return nil, err
	}
	switch major {
	case 5:
		c, err := v5.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		for _, opt := range opts {
			opt(c)
		}
		return c, nil
	case 6:
		c, err := v6.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		for _, opt := range opts {
			opt(c)
		}
		return c, nil
	default:
		return nil, errors.Errorf("unsupported Elasticsearch version %s", v)
	}
}

// elasticsearchVersion determines the Elasticsearch option.
func elasticsearchVersion(cfg *config.Config) (string, int64, int64, int64, error) {
	type infoType struct {
		Name    string `json:"name"`
		Version struct {
			Number string `json:"number"` // e.g. "6.2.4"
		} `json:"version"`
	}
	req, err := http.NewRequest("GET", cfg.URL, nil)
	if err != nil {
		return "", 0, 0, 0, err
	}
	if cfg.Username != "" || cfg.Password != "" {
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, 0, 0, err
	}
	defer res.Body.Close()
	var info infoType
	if err = json.NewDecoder(res.Body).Decode(&info); err != nil {
		return "", 0, 0, 0, err
	}
	v, err := semver.NewVersion(info.Version.Number)
	if err != nil {
		return info.Version.Number, 0, 0, 0, err
	}
	return info.Version.Number, v.Major(), v.Minor(), v.Patch(), nil
}
