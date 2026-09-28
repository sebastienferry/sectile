package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

// This acceptance check installs Vite inside a production-generated checkout.
// Run with SECTILE_VITE_ACCEPTANCE=1 and PLAYWRIGHT_MODULE pointing to index.mjs.
func TestGeneratedWorktreeLoadsViteInBrowser(t *testing.T) {
	if os.Getenv("SECTILE_VITE_ACCEPTANCE") != "1" {
		t.Skip("opt-in browser acceptance check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	files := map[string]string{
		"package.json": `{"private":true,"type":"module","devDependencies":{"vite":"8.2.0"}}`,
		"index.html":   `<div id="app"></div><script type="module" src="/src/main.js"></script>`,
		"src/main.js":  `document.querySelector('#app').textContent='safe-worktree-loaded'`,
	}
	for name, raw := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
	}
	gitTest(t, root, "init", "-q", "-b", "main")
	gitTest(t, root, "add", ".")
	gitTest(t, root, "commit", "-q", "-m", "fixture")
	branch := "feat/289"
	path, _, err := ensureLocalWorktree(ctx, root, models.Task{Key: "#289", BranchName: &branch}, true)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "issue-289" {
		t.Fatalf("unexpected generated path %s", path)
	}
	npm := exec.CommandContext(ctx, "npm", "install", "--no-audit", "--no-fund")
	npm.Dir = path
	if out, err := npm.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if info, err := os.Lstat(filepath.Join(path, "node_modules", "vite")); err != nil || !info.IsDir() {
		t.Fatal("Vite dependencies must be local")
	}
	script := filepath.Join(root, "browser.mjs")
	if err := os.WriteFile(script, []byte(`
import assert from 'node:assert/strict';
import { pathToFileURL } from 'node:url';
const root = process.argv[2];
const { createServer } = await import(pathToFileURL(root+'/node_modules/vite/dist/node/index.js'));
const { chromium } = await import(pathToFileURL(process.env.PLAYWRIGHT_MODULE));
const server = await createServer({root,server:{host:'127.0.0.1',port:0}});
let browser;
try {
 await server.listen();
 browser = await chromium.launch({headless:true,channel:'chrome'});
 const page = await browser.newPage();
 const errors=[];page.on('pageerror',e=>errors.push(e.message));
 const moduleResponse=page.waitForResponse(r=>r.url().endsWith('/src/main.js'));
 await page.goto('http://127.0.0.1:'+server.httpServer.address().port);
 assert.equal((await moduleResponse).status(),200);
 await page.waitForFunction(()=>document.querySelector('#app')?.textContent==='safe-worktree-loaded');
 assert.deepEqual(errors,[]);
 console.log('PASS: source module HTTP 200, DOM safe-worktree-loaded; root='+root+'; dependencies='+root+'/node_modules');
} finally { if(browser) await browser.close(); await server.close(); }
`), 0644); err != nil {
		t.Fatal(err)
	}
	node := exec.CommandContext(ctx, "node", script, path)
	node.Dir = path
	out, err := node.CombinedOutput()
	if err != nil {
		t.Fatalf("browser: %v\n%s", err, out)
	}
	t.Log(string(out))
}
