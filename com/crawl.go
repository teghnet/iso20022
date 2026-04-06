package com

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gocolly/colly/v2"
)

func Crawl(args []string) error {
	dir := args[0]
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}
	fmt.Printf("Writing files to '%s'\n", dir)

	// Instantiate default collector
	c := colly.NewCollector(
		colly.AllowedDomains("www.iso20022.org"),
		colly.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"),
		colly.CacheDir("cache"),
	)

	// Add headers on all requests
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Sec-GPC", "1")
		r.Headers.Set("Sec-Fetch-Dest", "document")
		r.Headers.Set("Sec-Fetch-Mode", "navigate")
		r.Headers.Set("Sec-Fetch-Site", "none")
		r.Headers.Set("Sec-Fetch-User", "?1")
	})

	// Visit all download links of the current ISO 20022 message catalogue
	c.OnHTML("a[href$=download][aria-label=Download]", func(e *colly.HTMLElement) {
		fmt.Printf("AAA %s\n", e.Attr("href"))
		err := e.Request.Visit(e.Attr("href"))
		if err != nil {
			fmt.Printf("Failed to visit %s: %s\n", e.Attr("href"), err)
		}
	})

	// Visit all download links of the ISO 20022 message catalogue archive
	c.OnHTML("a[href$=download][aria-label='Download Complete Message Set']", func(e *colly.HTMLElement) {
		fmt.Printf("BBB %s\n", e.Attr("href"))
		err := e.Request.Visit(e.Attr("href"))
		if err != nil {
			fmt.Printf("Failed to visit %s: %s\n", e.Attr("href"), err)
		}
	})

	// Handle paging
	c.OnHTML("a[href][title^='Go to page']", func(e *colly.HTMLElement) {
		fmt.Printf("CCC next page\n")
		err := e.Request.Visit(e.Attr("href"))
		if err != nil {
			fmt.Printf("Failed to visit %s: %s\n", e.Attr("href"), err)
		}
	})

	// Download message catalogue zip files
	c.OnResponse(func(r *colly.Response) {
		fmt.Printf("Response received: %s\n", r.Request.URL)
		if strings.Index(r.Headers.Get("Content-Type"), "application/zip") > -1 {
			fmt.Printf("Processing %s\n", r.FileName())
			if err := unzipWriteFile(r.Body, dir); err != nil {
				fmt.Printf("Failed to unzip file '%s': %s\n", r.FileName(), err)
			}
		}
	})

	err = c.Visit("https://www.iso20022.org/iso-20022-message-definitions?page=0")
	if err != nil {
		fmt.Printf("Failed to visit %s: %s\n", "https://www.iso20022.org/iso-20022-message-definitions?page=0", err)
	}
	err = c.Visit("https://www.iso20022.org/catalogue-messages/iso-20022-messages-archive?page=0")
	if err != nil {
		fmt.Printf("Failed to visit %s: %s\n", "https://www.iso20022.org/catalogue-messages/iso-20022-messages-archive?page=0", err)
	}
	return nil
}

func unzipWriteFile(data []byte, dir string) error {
	mime, err := mimetype.DetectReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to detect mimetype")
	}
	if !mime.Is("application/zip") {
		return fmt.Errorf("data is not a zipfile")
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to read zipped data: %s (%s)", err, mime)
	}

	// Read all the files from zip archive
	for _, zipFile := range zipReader.File {
		// we are only interested in xml schema and zip files
		if path.Ext(zipFile.Name) != ".xsd" && path.Ext(zipFile.Name) != ".zip" {
			fmt.Printf("ignoring[%s] %s\n", path.Ext(zipFile.Name), zipFile.Name)
			continue
		}
		fmt.Printf("  extracting %s\n", zipFile.Name)
		unzippedFileBytes, err := readZipFile(zipFile)
		if err != nil {
			return fmt.Errorf("failed to unzip file '%s': %s", zipFile.Name, err)
		}

		if path.Ext(zipFile.Name) == ".zip" {
			if err := unzipWriteFile(unzippedFileBytes, dir); err != nil {
				return err
			}
			continue
		}

		// store the extracted file
		if err := ioutil.WriteFile(fmt.Sprintf("%s/%s", dir, zipFile.Name), unzippedFileBytes, 0644); err != nil {
			return fmt.Errorf("failed to save extracted file as '%s/%s': %s", dir, zipFile.Name, err)
		}
	}

	return nil
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}
