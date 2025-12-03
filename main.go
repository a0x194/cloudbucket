package main

import (
	"bufio"
	"crypto/tls"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	version = "1.0.0"
	banner  = `
 ██████╗██╗      ██████╗ ██╗   ██╗██████╗ ██████╗ ██╗   ██╗ ██████╗██╗  ██╗███████╗████████╗
██╔════╝██║     ██╔═══██╗██║   ██║██╔══██╗██╔══██╗██║   ██║██╔════╝██║ ██╔╝██╔════╝╚══██╔══╝
██║     ██║     ██║   ██║██║   ██║██║  ██║██████╔╝██║   ██║██║     █████╔╝ █████╗     ██║
██║     ██║     ██║   ██║██║   ██║██║  ██║██╔══██╗██║   ██║██║     ██╔═██╗ ██╔══╝     ██║
╚██████╗███████╗╚██████╔╝╚██████╔╝██████╔╝██████╔╝╚██████╔╝╚██████╗██║  ██╗███████╗   ██║
 ╚═════╝╚══════╝ ╚═════╝  ╚═════╝ ╚═════╝ ╚═════╝  ╚═════╝  ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝

    Cloud Storage Bucket Scanner v%s

    Author: a0x194
    Team:   TryHarder | https://www.tryharder.space
    Tools:  https://www.tryharder.space/tools/
`
)

type Provider string

const (
	AWS       Provider = "AWS S3"
	GCP       Provider = "Google Cloud Storage"
	Azure     Provider = "Azure Blob"
	Alibaba   Provider = "Alibaba OSS"
	DigitalOcean Provider = "DigitalOcean Spaces"
)

type BucketResult struct {
	Name        string
	Provider    Provider
	URL         string
	Exists      bool
	PublicRead  bool
	PublicWrite bool
	PublicList  bool
	Files       []string
	Error       string
}

type Scanner struct {
	client      *http.Client
	verbose     bool
	checkWrite  bool
	listFiles   bool
	maxFiles    int
}

func NewScanner(timeout int, verbose bool, checkWrite bool, listFiles bool, maxFiles int) *Scanner {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &Scanner{
		client: &http.Client{
			Transport: tr,
			Timeout:   time.Duration(timeout) * time.Second,
		},
		verbose:    verbose,
		checkWrite: checkWrite,
		listFiles:  listFiles,
		maxFiles:   maxFiles,
	}
}

// AWS S3 bucket formats
func (s *Scanner) checkAWS(bucketName string) *BucketResult {
	result := &BucketResult{
		Name:     bucketName,
		Provider: AWS,
	}

	// Try path-style URL first
	urls := []string{
		fmt.Sprintf("https://%s.s3.amazonaws.com", bucketName),
		fmt.Sprintf("https://s3.amazonaws.com/%s", bucketName),
	}

	for _, url := range urls {
		result.URL = url

		// Check if bucket exists and is listable
		resp, err := s.client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		result.Exists = true

		if resp.StatusCode == 200 {
			result.PublicList = true
			result.PublicRead = true

			if s.listFiles {
				body, _ := io.ReadAll(resp.Body)
				result.Files = s.parseS3Listing(string(body))
			}
		} else if resp.StatusCode == 403 {
			// Bucket exists but not listable, try to read a file
			testURL := url + "/test.txt"
			testResp, err := s.client.Get(testURL)
			if err == nil {
				defer testResp.Body.Close()
				if testResp.StatusCode != 403 {
					result.PublicRead = true
				}
			}
		}

		// Check write access if enabled
		if s.checkWrite && result.Exists {
			result.PublicWrite = s.checkWriteAccess(url)
		}

		if result.Exists {
			return result
		}
	}

	return result
}

// Google Cloud Storage
func (s *Scanner) checkGCP(bucketName string) *BucketResult {
	result := &BucketResult{
		Name:     bucketName,
		Provider: GCP,
		URL:      fmt.Sprintf("https://storage.googleapis.com/%s", bucketName),
	}

	resp, err := s.client.Get(result.URL)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		result.Exists = true
		result.PublicList = true
		result.PublicRead = true

		if s.listFiles {
			body, _ := io.ReadAll(resp.Body)
			result.Files = s.parseGCSListing(string(body))
		}
	} else if resp.StatusCode == 403 {
		result.Exists = true
	}

	if s.checkWrite && result.Exists {
		result.PublicWrite = s.checkWriteAccess(result.URL)
	}

	return result
}

// Azure Blob Storage
func (s *Scanner) checkAzure(bucketName string) *BucketResult {
	result := &BucketResult{
		Name:     bucketName,
		Provider: Azure,
		URL:      fmt.Sprintf("https://%s.blob.core.windows.net", bucketName),
	}

	// Azure requires container name, try common ones
	containers := []string{"$web", "public", "data", "files", "backup", "images", "static"}

	for _, container := range containers {
		url := fmt.Sprintf("%s/%s?restype=container&comp=list", result.URL, container)

		resp, err := s.client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			result.Exists = true
			result.PublicList = true
			result.PublicRead = true
			result.URL = fmt.Sprintf("%s/%s", result.URL, container)

			if s.listFiles {
				body, _ := io.ReadAll(resp.Body)
				result.Files = s.parseAzureListing(string(body))
			}
			return result
		} else if resp.StatusCode == 403 {
			result.Exists = true
		}
	}

	return result
}

// Alibaba OSS
func (s *Scanner) checkAlibaba(bucketName string) *BucketResult {
	result := &BucketResult{
		Name:     bucketName,
		Provider: Alibaba,
	}

	regions := []string{
		"oss-cn-hangzhou", "oss-cn-shanghai", "oss-cn-beijing",
		"oss-cn-shenzhen", "oss-cn-hongkong", "oss-us-west-1",
	}

	for _, region := range regions {
		url := fmt.Sprintf("https://%s.%s.aliyuncs.com", bucketName, region)
		result.URL = url

		resp, err := s.client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			result.Exists = true
			result.PublicList = true
			result.PublicRead = true

			if s.listFiles {
				body, _ := io.ReadAll(resp.Body)
				result.Files = s.parseS3Listing(string(body)) // OSS uses S3-compatible format
			}
			return result
		} else if resp.StatusCode == 403 {
			result.Exists = true
			return result
		}
	}

	return result
}

// DigitalOcean Spaces
func (s *Scanner) checkDigitalOcean(bucketName string) *BucketResult {
	result := &BucketResult{
		Name:     bucketName,
		Provider: DigitalOcean,
	}

	regions := []string{"nyc3", "ams3", "sgp1", "fra1", "sfo2", "sfo3"}

	for _, region := range regions {
		url := fmt.Sprintf("https://%s.%s.digitaloceanspaces.com", bucketName, region)
		result.URL = url

		resp, err := s.client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			result.Exists = true
			result.PublicList = true
			result.PublicRead = true

			if s.listFiles {
				body, _ := io.ReadAll(resp.Body)
				result.Files = s.parseS3Listing(string(body))
			}
			return result
		} else if resp.StatusCode == 403 {
			result.Exists = true
			return result
		}
	}

	return result
}

func (s *Scanner) checkWriteAccess(baseURL string) bool {
	testFile := fmt.Sprintf("%s/cloudbucket_test_%d.txt", baseURL, time.Now().Unix())

	req, err := http.NewRequest("PUT", testFile, strings.NewReader("cloudbucket security test"))
	if err != nil {
		return false
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		// Clean up - try to delete the test file
		delReq, _ := http.NewRequest("DELETE", testFile, nil)
		s.client.Do(delReq)
		return true
	}

	return false
}

func (s *Scanner) parseS3Listing(body string) []string {
	var files []string

	type Contents struct {
		Key string `xml:"Key"`
	}
	type ListBucketResult struct {
		Contents []Contents `xml:"Contents"`
	}

	var result ListBucketResult
	xml.Unmarshal([]byte(body), &result)

	for i, c := range result.Contents {
		if i >= s.maxFiles {
			break
		}
		files = append(files, c.Key)
	}

	return files
}

func (s *Scanner) parseGCSListing(body string) []string {
	// GCS returns similar XML format
	return s.parseS3Listing(body)
}

func (s *Scanner) parseAzureListing(body string) []string {
	var files []string

	type Blob struct {
		Name string `xml:"Name"`
	}
	type Blobs struct {
		Blob []Blob `xml:"Blob"`
	}
	type EnumerationResults struct {
		Blobs Blobs `xml:"Blobs"`
	}

	var result EnumerationResults
	xml.Unmarshal([]byte(body), &result)

	for i, b := range result.Blobs.Blob {
		if i >= s.maxFiles {
			break
		}
		files = append(files, b.Name)
	}

	return files
}

func (s *Scanner) ScanBucket(bucketName string, providers []string) []BucketResult {
	var results []BucketResult

	if s.verbose {
		fmt.Printf("[*] Scanning bucket: %s\n", bucketName)
	}

	for _, provider := range providers {
		var result *BucketResult

		switch strings.ToLower(provider) {
		case "aws", "s3":
			result = s.checkAWS(bucketName)
		case "gcp", "gcs", "google":
			result = s.checkGCP(bucketName)
		case "azure":
			result = s.checkAzure(bucketName)
		case "alibaba", "aliyun", "oss":
			result = s.checkAlibaba(bucketName)
		case "do", "digitalocean", "spaces":
			result = s.checkDigitalOcean(bucketName)
		case "all":
			results = append(results, *s.checkAWS(bucketName))
			results = append(results, *s.checkGCP(bucketName))
			results = append(results, *s.checkAzure(bucketName))
			results = append(results, *s.checkAlibaba(bucketName))
			results = append(results, *s.checkDigitalOcean(bucketName))
			continue
		}

		if result != nil {
			results = append(results, *result)
		}
	}

	return results
}

func printResult(r BucketResult) {
	if !r.Exists && !r.PublicRead && !r.PublicList {
		return
	}

	red := "\033[31m"
	green := "\033[32m"
	yellow := "\033[33m"
	cyan := "\033[36m"
	reset := "\033[0m"

	severity := yellow
	status := "EXISTS"
	if r.PublicWrite {
		severity = red
		status = "CRITICAL - PUBLIC WRITE"
	} else if r.PublicList {
		severity = red
		status = "HIGH - PUBLIC LIST"
	} else if r.PublicRead {
		severity = yellow
		status = "MEDIUM - PUBLIC READ"
	}

	fmt.Printf("\n%s[%s]%s %s\n", severity, status, reset, r.Name)
	fmt.Printf("  %s├─%s Provider: %s%s%s\n", green, reset, cyan, r.Provider, reset)
	fmt.Printf("  %s├─%s URL: %s\n", green, reset, r.URL)
	fmt.Printf("  %s├─%s Public Read: %v\n", green, reset, r.PublicRead)
	fmt.Printf("  %s├─%s Public List: %v\n", green, reset, r.PublicList)
	fmt.Printf("  %s├─%s Public Write: %v\n", green, reset, r.PublicWrite)

	if len(r.Files) > 0 {
		fmt.Printf("  %s└─%s Files found (%d):\n", green, reset, len(r.Files))
		for _, f := range r.Files {
			fmt.Printf("      • %s\n", f)
		}
	}
}

func main() {
	var (
		bucket      string
		list        string
		providers   string
		threads     int
		timeout     int
		verbose     bool
		checkWrite  bool
		listFiles   bool
		maxFiles    int
		output      string
		showVersion bool
	)

	flag.StringVar(&bucket, "b", "", "Single bucket name to scan")
	flag.StringVar(&list, "l", "", "File containing list of bucket names")
	flag.StringVar(&providers, "p", "all", "Providers to check: aws,gcp,azure,alibaba,do,all")
	flag.IntVar(&threads, "t", 10, "Number of concurrent threads")
	flag.IntVar(&timeout, "timeout", 10, "Request timeout in seconds")
	flag.BoolVar(&verbose, "v", false, "Verbose output")
	flag.BoolVar(&checkWrite, "write", false, "Check for write access (creates test file)")
	flag.BoolVar(&listFiles, "files", false, "List files in accessible buckets")
	flag.IntVar(&maxFiles, "max-files", 10, "Maximum files to list per bucket")
	flag.StringVar(&output, "o", "", "Output file for results")
	flag.BoolVar(&showVersion, "version", false, "Show version")

	flag.Parse()

	fmt.Printf(banner, version)

	if showVersion {
		return
	}

	if bucket == "" && list == "" {
		fmt.Println("\nUsage:")
		fmt.Println("  cloudbucket -b company-backup")
		fmt.Println("  cloudbucket -l buckets.txt -p aws,gcp")
		fmt.Println("  cloudbucket -b mydata -files -write")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
		fmt.Println("\nProviders: aws, gcp, azure, alibaba, do (DigitalOcean), all")
		return
	}

	providerList := strings.Split(providers, ",")
	scanner := NewScanner(timeout, verbose, checkWrite, listFiles, maxFiles)

	var buckets []string
	if bucket != "" {
		buckets = append(buckets, bucket)
	}

	if list != "" {
		file, err := os.Open(list)
		if err != nil {
			fmt.Printf("[!] Error opening file: %v\n", err)
			return
		}
		defer file.Close()

		sc := bufio.NewScanner(file)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				buckets = append(buckets, line)
			}
		}
	}

	fmt.Printf("\n[*] Scanning %d bucket(s) across providers: %s\n", len(buckets), providers)

	var allResults []BucketResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, threads)

	for _, b := range buckets {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(bucketName string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			results := scanner.ScanBucket(bucketName, providerList)

			mu.Lock()
			for _, r := range results {
				if r.Exists || r.PublicRead || r.PublicList {
					allResults = append(allResults, r)
					printResult(r)
				}
			}
			mu.Unlock()
		}(b)
	}

	wg.Wait()

	vulnerable := 0
	for _, r := range allResults {
		if r.PublicWrite || r.PublicList || r.PublicRead {
			vulnerable++
		}
	}

	fmt.Printf("\n[*] Scan complete! Found %d accessible bucket(s)\n", vulnerable)

	// Save to file
	if output != "" && len(allResults) > 0 {
		file, err := os.Create(output)
		if err != nil {
			fmt.Printf("[!] Error creating output file: %v\n", err)
			return
		}
		defer file.Close()

		for _, r := range allResults {
			line := fmt.Sprintf("%s | %s | %s | Read:%v | List:%v | Write:%v\n",
				r.Name, r.Provider, r.URL, r.PublicRead, r.PublicList, r.PublicWrite)
			file.WriteString(line)
		}
		fmt.Printf("[*] Results saved to %s\n", output)
	}
}
