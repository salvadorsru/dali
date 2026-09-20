package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/salvadorsru/dali/embed"
	"github.com/salvadorsru/dali/output"
	"github.com/salvadorsru/dali/query"
	"github.com/salvadorsru/dali/semantic"
)

// cmdIndex builds the index for a fig. By default it only builds the structural
// index (SQLite + FTS5); pass --embeddings to also build the vector index.
func cmdIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ContinueOnError)
	embeddings := fs.Bool("embeddings", false, "also build the semantic (embedding) index")
	force := fs.Bool("force", false, "rebuild even if the index is up to date")
	yes := fs.Bool("yes", false, "confirm downloads without prompting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *yes {
		os.Setenv("DALI_EMBED_YES", "1")
	}

	status := output.Start("indexing " + filepath.Base(figFile))
	svc, err := query.OpenWith(figFile, *force)
	status.Stop()
	if err != nil {
		return err
	}
	defer svc.Close()

	printIndex(svc)

	if !*embeddings {
		fmt.Fprintln(os.Stderr, output.Dim("embeddings: skipped (use --embeddings)"))
		return nil
	}
	if !svc.DB().VecAvailable() {
		return fmt.Errorf("sqlite-vec is not available in this build")
	}
	if *force {
		// --force rebuilds what was asked for: drop the vector index too and
		// re-embed everything below.
		if err := svc.DB().DropVec(); err != nil {
			return err
		}
		if err := svc.DB().ClearEmbeddings(); err != nil {
			return err
		}
	}

	emb := embed.FromEnv()
	defer embed.Close(emb)
	stats, err := semantic.IndexWithProgress(context.Background(), svc.DB(), emb, progress("embeddings"))
	if err != nil {
		return err
	}
	fmt.Printf("\n%s %s %s\n", output.Label("embeddings:"), output.Bold(stats.Model), output.Dim(fmt.Sprintf("%dd", stats.Dimensions)))
	fmt.Printf("            %s %d   %s %d   %s %d   %s %d\n",
		output.Dim("indexed"), stats.Indexed,
		output.Dim("reused"), stats.Reused,
		output.Dim("deleted"), stats.Deleted,
		output.Dim("skipped"), stats.Skipped)
	return nil
}

func kv(label, value string) {
	fmt.Printf("%s %s\n", output.Label(fmt.Sprintf("%-11s", label)), value)
}

func printIndex(svc *query.Service) {
	name, _ := svc.Document()
	counts, _ := svc.Counts()
	pages, _ := svc.Pages()
	components, _ := svc.Components()
	instances, _ := svc.Instances()
	assets, _ := svc.Assets()
	fonts, _ := svc.Fonts()
	styles, _ := svc.Styles()

	nodes := 0
	for _, n := range counts {
		nodes += n
	}

	fmt.Println(output.Title(name))
	kv("pages:", fmt.Sprintf("%d", len(pages)))
	kv("nodes:", fmt.Sprintf("%d", nodes))
	kv("components:", fmt.Sprintf("%d", len(components)))
	kv("instances:", fmt.Sprintf("%d", len(instances)))
	kv("images:", fmt.Sprintf("%d", len(assets)))
	kv("fonts:", fmt.Sprintf("%d", len(fonts)))
	kv("styles:", fmt.Sprintf("%d", len(styles)))
	status := output.Success("up to date")
	if svc.Rebuilt() {
		status = output.Warn("rebuilt")
	}
	kv("index:", status)
}

func cmdSemanticSearch(args []string) error {
	q, err := arg(args, 0, "query")
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("semantic-search", flag.ContinueOnError)
	limit := fs.Int("limit", 20, "max results")
	typ := fs.String("type", "", "filter by node type")
	page := fs.String("page", "", "filter by page name or id")
	hybrid := fs.Bool("hybrid", false, "combine with full-text search (RRF)")
	yes := fs.Bool("yes", false, "confirm downloads without prompting")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *yes {
		os.Setenv("DALI_EMBED_YES", "1")
	}

	svc, err := open(figFile)
	if err != nil {
		return err
	}
	defer svc.Close()

	if !svc.DB().VecAvailable() {
		return fmt.Errorf("sqlite-vec is not available in this build")
	}
	emb := embed.FromEnv()
	defer embed.Close(emb)
	if stats, err := semantic.IndexWithProgress(context.Background(), svc.DB(), emb, progress("embeddings")); err != nil {
		return err
	} else if stats.Indexed > 0 {
		fmt.Fprintf(os.Stderr, "dali: embedded %d node(s)\n", stats.Indexed)
	}

	opts := query.Options{Query: q, Type: *typ, Limit: *limit, Mode: query.ModeSemantic}
	if *page != "" {
		id, err := svc.PageByName(*page)
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("page %q not found", *page)
		}
		opts.PageID = id
	}

	var results []query.Result
	if *hybrid {
		results, err = svc.HybridSearch(context.Background(), emb, opts)
	} else {
		results, err = svc.SemanticSearch(context.Background(), emb, opts)
	}
	if err != nil {
		return err
	}
	if jsonOut {
		return output.JSON(os.Stdout, results)
	}
	rows := make([][]string, 0, len(results))
	for _, r := range results {
		rows = append(rows, []string{output.ID(r.ID), output.Type(r.Type), output.Success(fmt.Sprintf("%.3f", r.Score)), truncate(r.Name, 40), output.Dim(truncate(r.Path, 60))})
	}
	output.Table(os.Stdout, []string{"ID", "TYPE", "SCORE", "NAME", "PATH"}, rows)
	return nil
}

func progress(label string) semantic.Progress {
	var bar *output.Progress
	return func(done, total int) {
		if bar == nil {
			bar = output.NewProgress(label, total)
		}
		if done >= total {
			bar.Done()
		} else {
			bar.Set(done)
		}
	}
}
