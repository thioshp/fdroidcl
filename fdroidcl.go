/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Repo struct {
	Apps []App `xml:"application"`
}

type App struct {
	Name    string `xml:"name"`
	ID      string `xml:"id"`
	Summary string `xml:"summary"`
	Desc    string `xml:"desc"`
	License string `xml:"license"`
	Categs  string `xml:"categories"`
	CVName  string `xml:"marketversion"`
	CVCode  uint   `xml:"marketvercode"`
	Website string `xml:"web"`
	Source  string `xml:"source"`
	Tracker string `xml:"tracker"`
	Apks    []Apk  `xml:"package"`
	CurApk  *Apk
}

type Apk struct {
	VName   string `xml:"version"`
	VCode   uint   `xml:"versioncode"`
	ApkName string `xml:"apkname"`
	SrcName string `xml:"srcname"`
	Size    int    `xml:"size"`
	MinSdk  int    `xml:"sdkver"`
}

func Form(f, str string) string { return fmt.Sprintf("\033[%sm%s\033[0m", f, str) }
func Bold(str string) string    { return Form("1", str) }
func Green(str string) string   { return Form("1;32", str) }
func Blue(str string) string    { return Form("1;34", str) }
func Purple(str string) string  { return Form("1;35", str) }

func (app *App) prepareData() {
	for _, apk := range app.Apks {
		app.CurApk = &apk
		if app.CVCode >= apk.VCode {
			break
		}
	}
}

func (app *App) WriteShort(w io.Writer) {
	fmt.Fprintf(w, "%s %s %s\n", Bold(app.Name),
		Purple(app.ID), Green(app.CurApk.VName))
	fmt.Fprintf(w, "    %s\n", app.Summary)
}

func (app *App) WriteDetailed(w io.Writer) {
	p := func(title string, format string, args ...interface{}) {
		if format == "" {
			fmt.Fprintln(w, Bold(title))
		} else {
			fmt.Fprintf(w, "%s %s\n", Bold(title), fmt.Sprintf(format, args...))
		}
	}
	p("Name             :", "%s", app.Name)
	p("Summary          :", "%s", app.Summary)
	p("Current Version  :", "%s (%d)", app.CurApk.VName, app.CurApk.VCode)
	p("Upstream Version :", "%s (%d)", app.CVName, app.CVCode)
	p("License          :", "%s", app.License)
	if app.Categs != "" {
		p("Categories       :", "%s", app.Categs)
	}
	if app.Website != "" {
		p("Website          :", "%s", app.Website)
	}
	if app.Source != "" {
		p("Source           :", "%s", app.Source)
	}
	if app.Tracker != "" {
		p("Tracker          :", "%s", app.Tracker)
	}
	// p("Description     :", "%s", app.Desc) TODO: html, 80 column wrapping
	fmt.Println()
	p("Available Versions :", "")
	for _, apk := range app.Apks {
		fmt.Println()
		p("    Name   :", "%s (%d)", apk.VName, apk.VCode)
		p("    Size   :", "%d", apk.Size)
		p("    MinSdk :", "%d", apk.MinSdk)
	}
}

const indexName = "index.jar"

var repoURL = flag.String("r", "https://f-droid.org/repo", "repository address")

func updateIndex() {
	url := fmt.Sprintf("%s/%s", *repoURL, indexName)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to fetch '%s': %s", url, err)
	}
	defer resp.Body.Close()
	out, err := os.Create(indexName)
	if err != nil {
		log.Fatalf("Failed to create file '%s': %s", indexName, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		log.Fatal(err)
	}
}

func loadApps() map[string]App {
	r, err := zip.OpenReader(indexName)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	buf := new(bytes.Buffer)

	for _, f := range r.File {
		if f.Name != "index.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		if _, err = io.Copy(buf, rc); err != nil {
			log.Fatal(err)
		}
		rc.Close()
		break
	}

	var repo Repo
	if err := xml.Unmarshal(buf.Bytes(), &repo); err != nil {
		log.Fatalf("Could not read xml: %s", err)
	}
	apps := make(map[string]App)

	for i := range repo.Apps {
		app := repo.Apps[i]
		app.prepareData()
		apps[app.ID] = app
	}
	return apps
}

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	cmd := flag.Args()[0]
	args := flag.Args()[1:]

	switch cmd {
	case "update":
		updateIndex()
	case "list":
		apps := loadApps()
		for _, app := range apps {
			app.WriteShort(os.Stdout)
		}
	case "show":
		apps := loadApps()
		for _, appID := range args {
			app, e := apps[appID]
			if !e {
				log.Fatalf("Could not find app with ID '%s'", appID)
			}
			app.WriteDetailed(os.Stdout)
		}
	default:
		log.Fatalf("Unrecognised command '%s'", cmd)
	}
}
