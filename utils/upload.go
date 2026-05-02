package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// UploadToPages 把当前 result.csv + 同目录的 index.html 一起部署到
// Cloudflare Pages 项目 projectName。projectName 为空则跳过。
// index.html 必须与 result.csv (-o 指定的输出文件) 在同一目录。
// 依赖外部 wrangler CLI（首次使用前 npm i -g wrangler && wrangler login）。
func UploadToPages(projectName string) error {
	if projectName == "" {
		return nil
	}
	if noOutput() {
		return errors.New("未配置 -o 输出文件，跳过 Pages 上传")
	}
	if _, err := os.Stat(Output); err != nil {
		return fmt.Errorf("找不到结果文件 %s: %w", Output, err)
	}

	indexPath := filepath.Join(filepath.Dir(Output), "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return fmt.Errorf("找不到 %s（请确保 index.html 与 %s 同目录）: %w", indexPath, Output, err)
	}

	const dist = "deploy/pages"
	if err := os.MkdirAll(dist, 0755); err != nil {
		return fmt.Errorf("创建 %s 目录失败: %w", dist, err)
	}

	if err := copyFile(Output, filepath.Join(dist, "result.csv")); err != nil {
		return fmt.Errorf("复制 %s 失败: %w", Output, err)
	}
	if err := copyFile(indexPath, filepath.Join(dist, "index.html")); err != nil {
		return fmt.Errorf("复制 %s 失败: %w", indexPath, err)
	}

	Cyan.Printf("\n开始部署到 Cloudflare Pages 项目 [%s]...\n", projectName)
	// --branch=main 让 wrangler 把本次部署标记到 CF Pages 项目的 Production Branch
	// （默认就是 main），主域名 https://<project>.pages.dev 才会更新。
	// 否则 wrangler 会用本地 git 分支名（如 master）当 branch 走 preview，
	// 主域名内容不变，只能通过 https://<branch>.<project>.pages.dev 看到新版。
	cmd := exec.Command("wrangler", "pages", "deploy", dist,
		"--project-name="+projectName,
		"--branch=main",
		"--commit-dirty=true",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return errors.New("未找到 wrangler，请先 npm install -g wrangler && wrangler login")
		}
		return fmt.Errorf("wrangler 部署失败: %w", err)
	}
	Green.Printf("✓ 部署完成：https://%s.pages.dev\n", projectName)
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
