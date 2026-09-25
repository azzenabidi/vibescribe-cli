package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/azzenabidi/vibescribe-cli/pkg/model"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

func newModelCmd() *cobra.Command {
	modelCmd := &cobra.Command{
		Use:   "model",
		Short: "Manage Whisper GGML models",
		Long: `VibeScribe downloads Whisper GGML models from the whisper.cpp
repository on Hugging Face into the model cache directory.`,
	}
	modelCmd.AddCommand(
		newModelDownloadCmd(),
		newModelListCmd(),
	)
	return modelCmd
}

func newModelDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "download [tiny|base|small|medium|large-v3]",
		Aliases:   []string{"get"},
		Short:     "Download a Whisper GGML model",
		Long:      `Download a Whisper GGML model (default: base) from Hugging Face into the model cache.`,
		Args:      cobra.MaximumNArgs(1),
		RunE:      runModelDownload,
		ValidArgs: model.Names(),
	}
	cmd.Flags().Bool("yes", false, "skip the download confirmation prompt")
	return cmd
}

func runModelDownload(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	yes, _ := cmd.Flags().GetBool("yes")

	name := "base"
	if len(args) == 1 {
		name = args[0]
	}
	if !model.ValidNames(name) {
		return fmt.Errorf("unknown model %q (known sizes: %s)", name, joinNames(model.Names()))
	}

	dest := filepath.Join(model.CacheDir(), model.File(name))
	if model.Exists(dest) {
		fmt.Fprintf(os.Stderr, "Already cached: %s\n", dest)
		return nil
	}

	sizeMB := ""
	if size, err := model.RemoteSize(name); err == nil && size > 0 {
		sizeMB = fmt.Sprintf(" (~%.1f MB)", float64(size)/(1024*1024))
	}
	fmt.Fprintf(os.Stderr, "Model %q is not in the cache.%s\n", name, sizeMB)

	if !yes {
		ok, err := confirm("Download it now?", false)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted download of model %q", name)
		}
	}

	if err := downloadModel(ctx, name, dest); err != nil {
		return err
	}

	if info, err := os.Stat(dest); err == nil {
		fmt.Fprintf(os.Stderr, "Saved %s (%s)\n", dest, prettySize(info.Size()))
	}
	return nil
}

func confirm(prompt string, defTrue bool) (bool, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return defTrue, nil
	}

	var hint string
	if defTrue {
		hint = " [Y/n] "
	} else {
		hint = " [y/N] "
	}
	fmt.Fprintf(os.Stderr, "%s%s", prompt, hint)

	var reply string
	buf := make([]byte, 1)
	var out []byte
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
			if buf[0] == '\n' || buf[0] == '\r' {
				break
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
	}
	reply = string(out)
	switch reply {
	case "":
		return defTrue, nil
	case "y", "Y", "yes", "Yes", "YES":
		return true, nil
	default:
		return false, nil
	}
}

func downloadModel(ctx context.Context, name, dest string) error {
	total, _ := model.RemoteSize(name)

	var bar *progressbar.ProgressBar
	if total > 0 {
		bar = progressbar.NewOptions64(
			total,
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetDescription("Downloading "+name+" model"),
		)
	} else {
		bar = progressbar.NewOptions64(
			-1,
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetDescription("Downloading "+name+" model"),
		)
	}

	err := model.Download(ctx, name, dest, func(done, _ int64) {
		if total > 0 {
			_ = bar.Set64(done)
		}
	})
	if err != nil {
		bar.Clear()
		return err
	}
	bar.Finish()
	return nil
}

func newModelListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List known model sizes and locally cached models",
		RunE:  runModelList,
	}
}

func runModelList(cmd *cobra.Command, args []string) error {
	cached, err := model.Scan()
	if err == nil && len(cached.Files) > 0 {
		fmt.Fprintf(os.Stderr, "Cached models in %s:\n", model.CacheDir())
		w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
		for _, f := range cached.Files {
			fmt.Fprintf(w, "  %s\t%s\n", f.Name, prettySize(f.Size))
		}
		w.Flush()
	}

	w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Known model sizes:\tstatus")
	var names = model.Names()
	sort.Strings(names)
	for _, name := range names {
		dest := filepath.Join(model.CacheDir(), model.File(name))
		status := "not downloaded"
		if model.Exists(dest) {
			status = "downloaded"
		}
		fmt.Fprintf(w, "  %s\t%s\n", name, status)
	}
	w.Flush()
	return nil
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

func prettySize(bytes int64) string {
	const unit = 1024.0
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := float64(unit), 0
	for b := float64(bytes) / unit; b >= unit; b /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/div, "KMGTPE"[exp])
}
