package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/golem-mc/multiverse/core"
)

func init() { all = append(all, dumpsCommands) }

// PasteEndpoints are where /mv dumps uploads, by --upload service; tests
// point them at a local server. MV: PastesDevPasteService, McloGsPasteService
var PasteEndpoints = map[string]string{
	"pastesdev": "https://api.pastes.dev/post",
	"mclogs":    "https://api.mclo.gs/1/log",
}

// dumpsViewer shows an upload; Multiverse's links point there.
// MV: HttpAPIClient.DUMPS_VIEWER_URL
const dumpsViewer = "https://mvdumps.c0ding.party/"

// uploadServices are --upload's values and the names the result gives them.
// MV: DumpsLogPoster.UploadType
var uploadServices = map[string]string{"pastesdev": "pastes.dev", "mclogs": "mclo.gs"}

var started = time.Now()

// dumpsCommands is /mv dumps. MV: DumpsCommand, DumpsService, DumpsLogPoster
func dumpsCommands(c *core.Core) []*core.Cmd {
	return []*core.Cmd{{
		Name: "dumps", Permission: "multiverse.core.dumps", Syntax: "[--upload <pastesdev|mclogs>]",
		Description: "mv-core.dumps.description", MaxArgs: 0,
		Flags: []*core.Flag{{Name: "--upload", Aliases: []string{"-u"}, Value: true,
			Complete: func(*core.Ctx, string) []string { return []string{"pastesdev", "mclogs"} }}},
		Run: func(ctx *core.Ctx) error {
			service, err := pickOne(ctx.FlagOr("--upload", "pastesdev"), []string{"pastesdev", "mclogs"})
			if err != nil {
				return err
			}
			ctx.Info(core.M("mv-core.dumps.starting"))
			e := dumpsDebugInfo(ctx.Core)
			ctx.Core.Fire(e)
			payload := dumpsPayload(ctx.Core, e, serverLogs(ctx.Core))
			is := ctx.Issuer
			sched := ctx.Core.Server.Scheduler()
			sched.RunAsync(func() {
				link, err := uploadDumps(service, payload)
				if err != nil {
					ctx.Core.Log.Error("Failed to upload /mv dumps", "service", service, "error", err)
					link = "Error posting to service."
				}
				sched.RunTask(func() {
					is.Info(core.M("mv-core.dumps.url.list", "{service}", uploadServices[service], "{link}", link))
				})
			})
			return nil
		},
	}}
}

// serverDir is the server's working directory: two up from plugins/<name>.
func serverDir(c *core.Core) string { return filepath.Join(c.Server.DataDir(), "..", "..") }

// dumpsDebugInfo gathers the files a dump holds before other plugins add
// theirs. MV: DumpsService.addDebugInfoToEvent
func dumpsDebugInfo(c *core.Core) *core.MVDumpsDebugInfoEvent {
	e := &core.MVDumpsDebugInfoEvent{}
	var loaded, modules []string
	for _, w := range c.Worlds.Loaded() {
		loaded = append(loaded, w.Name())
	}
	for _, p := range c.Server.Plugins() {
		if strings.HasPrefix(p.Name, "multiverse-") {
			modules = append(modules, p.Name)
		}
	}
	version, _ := c.Server.Version()
	e.PutDetailedDebugInfo("version.md", "# Multiverse-Core Version info\n\n"+
		" - Multiverse-Core Version: "+core.Version+"\n"+
		" - Bukkit Version: golem ("+version+")\n"+
		" - Loaded Worlds: ["+strings.Join(loaded, ", ")+"]\n"+
		" - Multiverse Plugins Loaded: "+joinAnd(modules)+"\n"+
		fmt.Sprintf(" - Multiverse Plugins Count: %d\n", len(modules)))
	if data, err := os.ReadFile(filepath.Join(serverDir(c), "server.properties")); err == nil {
		e.PutDetailedDebugInfo("server.properties", string(data))
	} else {
		c.Log.Warn("/mv dumps could not find server.properties. Not including file")
	}
	for _, name := range []string{"config.yml", "worlds.yml", "anchors.yml"} {
		path := filepath.Join(c.Server.DataDir(), name)
		data, err := os.ReadFile(path)
		if err != nil {
			e.PutDetailedDebugInfo("multiverse-core/"+name, "ERROR: Could not load: "+path)
			continue
		}
		e.PutDetailedDebugInfo("multiverse-core/"+name, string(data))
	}
	return e
}

// joinAnd joins with commas and " and " before the last. MV: StringFormatter.joinAnd
func joinAnd(l []string) string {
	if len(l) < 2 {
		return strings.Join(l, "")
	}
	return strings.Join(l[:len(l)-1], ", ") + " and " + l[len(l)-1]
}

// serverLogs answers logs/latest.log. MV: DumpsService.getLogs
func serverLogs(c *core.Core) string {
	data, err := os.ReadFile(filepath.Join(serverDir(c), "logs", "latest.log"))
	if errors.Is(err, os.ErrNotExist) {
		c.Log.Warn("Could not read logs/latest.log")
		return "Could not find log"
	}
	if err != nil {
		return "Could not read log"
	}
	return string(data)
}

// dumpsPayload is the JSON Multiverse's dumps viewer reads: the server, its
// plugins and the files, the log last. MV: DumpsLogPoster.encodePasteAsJson
func dumpsPayload(c *core.Core, e *core.MVDumpsDebugInfoEvent, logs string) []byte {
	type file struct {
		Name string `json:"name"`
		Data string `json:"data"`
		Type string `json:"type"`
	}
	type pluginInfo struct {
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		Description string   `json:"description"`
		Authors     []string `json:"authors"`
		Website     string   `json:"website"`
		Enabled     bool     `json:"enabled"`
	}
	var files []file
	for _, f := range append(e.DetailedDebugInfo(), core.DumpFile{Name: "latest.log", Contents: logs}) {
		kind := "text"
		switch filepath.Ext(f.Name) {
		case ".yml", ".yaml":
			kind = "yaml"
		case ".json":
			kind = "json"
		case ".md":
			kind = "markdown"
		case ".log":
			kind = "accesslog"
		}
		files = append(files, file{f.Name, f.Contents, kind})
	}
	plugins := []pluginInfo{}
	for _, p := range c.Server.Plugins() {
		authors := []string{}
		if p.Name == "multiverse-core" {
			authors = core.Authors
		}
		plugins = append(plugins, pluginInfo{Name: p.Name, Version: p.Version, Authors: authors, Website: "", Enabled: p.Enabled})
	}
	version, _ := c.Server.Version()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	up := time.Since(started)
	payload := map[string]any{
		"server": map[string]any{
			"name": "golem", "version": version, "bukkitVersion": version,
			"javaVersion":     runtime.Version(),
			"operatingSystem": runtime.GOOS + " " + runtime.GOARCH,
			"uptime": fmt.Sprintf("%d hrs, %d mins, %d sec", int(up.Hours()), int(up.Minutes())%60,
				int(up.Seconds())%60),
			"memory": fmt.Sprintf("%d free / %d total / %d max", (mem.HeapIdle-mem.HeapReleased)>>20, mem.Sys>>20, mem.Sys>>20),
		},
		"plugins":   plugins,
		"files":     files,
		"createdAt": time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, _ := json.Marshal(payload)
	return data
}

// uploadDumps posts payload to service and answers the viewer's link.
// MV: PastesDevPasteService.postData, McloGsPasteService.postData
func uploadDumps(service string, payload []byte) (string, error) {
	body, contentType := payload, "application/json; charset=utf-8"
	if service == "mclogs" {
		body = []byte("content=" + url.QueryEscape(string(payload)))
		contentType = "application/x-www-form-urlencoded; charset=utf-8"
	}
	req, err := http.NewRequest(http.MethodPost, PasteEndpoints[service], bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "multiverse/dumps")
	req.Header.Set("Content-Type", contentType)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("%s: %s", resp.Status, data)
	}
	var answer struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	if err := json.Unmarshal(data, &answer); err != nil {
		return "", err
	}
	if service == "mclogs" {
		return dumpsViewer + answer.ID + "?service=mclogs", nil
	}
	return dumpsViewer + answer.Key, nil
}
