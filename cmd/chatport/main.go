// Command chatport moves AI conversation history between assistants.
//
// v0.1 (m1) implements the parse half of the happy path:
//
//	chatport parse <export.zip> [--stats] [--json]
//
// `chatport open` already serves the local placeholder page; the full
// drag-and-drop preview and Context Pack download land in v0.2 (m2).
package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/chatport/internal/ingest"
	"github.com/SuperMarioYL/chatport/internal/ir"
	"github.com/SuperMarioYL/chatport/internal/web"
)

// version is injected at build time: -ldflags "-X main.version=$(cat VERSION)".
var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "chatport",
		Short: "把 AI 助手的官方对话导出，变成下一个 Agent 一口吃下的上下文包",
		Long: `ChatPort 把官方导出的对话记录归一化为 Context Pack，
再把记忆带进下一个 Agent —— 搬家而非重来。

v0.1 支持解析 ChatGPT 官方导出 zip 并输出统计；
本地预览与上下文包渲染见 README 路线图。全程本地运行，不上传任何数据。`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("chatport {{.Version}}\n")
	root.AddCommand(parseCmd(), openCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "chatport:", err)
		os.Exit(1)
	}
}

func parseCmd() *cobra.Command {
	var wantStats, wantJSON bool
	cmd := &cobra.Command{
		Use:   "parse <export.zip>",
		Short: "读入官方导出 zip，归一化为 Context Pack 并输出统计",
		Long: `读入 AI 助手的官方导出 zip（v0.1 支持 ChatGPT 数据导出），
归一化为 Context Pack 中间表示并输出统计。

  chatport parse export.zip          概要统计
  chatport parse export.zip --stats  附每对话明细
  chatport parse export.zip --json   输出完整 Context Pack JSON`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pack, err := loadPack(args[0])
			if err != nil {
				return err
			}
			if wantJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				return enc.Encode(pack)
			}
			printStats(pack, wantStats)
			return nil
		},
	}
	cmd.Flags().BoolVar(&wantStats, "stats", false, "输出每对话消息数明细")
	cmd.Flags().BoolVar(&wantJSON, "json", false, "以 JSON 输出完整 Context Pack IR")
	return cmd
}

func openCmd() *cobra.Command {
	var addr string
	var noBrowser bool
	cmd := &cobra.Command{
		Use:   "open",
		Short: "启动 localhost 预览页（交互式预览在 v0.2 提供）",
		Long: `启动本地预览页并自动打开浏览器。

v0.1 说明：页面当前是占位预览；拖入 zip、勾选对话、一键下载
claude-projects-pack.zip 的完整交互在 v0.2 落地。全程本机回环，无任何上传。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			srv := web.New(version)
			listen, err := srv.Listen(addr)
			if err != nil {
				return err
			}
			url := "http://" + listen
			fmt.Printf("ChatPort %s 预览页已启动: %s （Ctrl+C 退出）\n", version, url)
			if !noBrowser {
				if err := web.OpenBrowser(url); err != nil {
					fmt.Println("（无法自动打开浏览器，请手动访问上面的地址）")
				}
			}
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8787", "监听地址（host:port）")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "不自动打开浏览器")
	return cmd
}

// loadPack detects the export format and ingests it into a Context Pack.
func loadPack(path string) (*ir.Pack, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开导出文件失败: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s 是目录，chatport 处理的是导出 zip 文件", path)
	}
	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("%s 不是有效的 zip 导出包: %w", path, err)
	}
	parser, err := ingest.Detect(zr)
	if err != nil {
		return nil, err
	}
	return parser.Parse(zr)
}

// printStats renders the terminal summary. Labels are zh-first to match
// the product audience; values stay machine-parseable.
func printStats(pack *ir.Pack, detailed bool) {
	s := pack.Stats()
	fmt.Printf("来源        %s 导出\n", s.SourceApp)
	fmt.Printf("对话数      %d\n", s.Conversations)
	if s.Duplicates > 0 {
		fmt.Printf("消息数      %d（其中重复 %d 条，渲染时将去重）\n", s.Messages, s.Duplicates)
	} else {
		fmt.Printf("消息数      %d\n", s.Messages)
	}
	fmt.Printf("时间跨度    %s\n", span(s.FirstAt, s.LastAt))
	grouped := 0
	for hint := range s.Projects {
		if hint != "" {
			grouped++
		}
	}
	fmt.Printf("项目分组    %d 个项目，未分组 %d 个对话\n", grouped, s.Projects[""])
	if !detailed {
		return
	}
	fmt.Println()
	fmt.Println("每对话消息数（“重复” = 与其他对话逐字相同、渲染时将去重的消息）:")
	for _, c := range s.PerConv {
		title := c.Title
		if title == "" {
			title = "（无标题）"
		}
		project := ir.ProjectLabel(c.ProjectHint)
		if c.Duplicates > 0 {
			fmt.Printf("  · %s  — %d 条（重复 %d）｜%s｜%s ~ %s\n",
				title, c.Messages, c.Duplicates, project, date(c.FirstAt), date(c.LastAt))
			continue
		}
		fmt.Printf("  · %s  — %d 条｜%s｜%s ~ %s\n",
			title, c.Messages, project, date(c.FirstAt), date(c.LastAt))
	}
}

func span(first, last time.Time) string {
	if first.IsZero() || last.IsZero() {
		return "—"
	}
	return first.Format("2006-01-02") + " ~ " + last.Format("2006-01-02")
}

func date(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("2006-01-02")
}
