package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type pathCheck struct {
	originalURL string
	finalURL    string
	baseURL     string
	path        string
}

var transport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	DialContext: (&net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: time.Second,
		DualStack: true,
	}).DialContext,
}

var httpClient = &http.Client{
	Transport: transport,
}

func main() {
	sc := bufio.NewScanner(os.Stdin)
	initialPathChecks := make(chan pathCheck, 100)
	donePaths := makePoolPath(initialPathChecks, func(c pathCheck, output chan pathCheck) {
		// Perform path reflection check on the final redirected URL
		reflected, basic, err := checkPathReflected(c.baseURL, c.path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error from checkPathReflected for url %s with path %s: %s\n", c.baseURL, c.path, err)
			return
		}
		if len(reflected) > 0 || basic != "" {
			displayURL := c.baseURL + c.path
			if c.originalURL != c.finalURL {
				fmt.Printf("URL: \033[34m%s\033[0m ==> \033[34m%s\033[0m\nReflected Characters: \033[31m%v\033[0m %s\n", c.originalURL, displayURL, reflected, basic)
			} else {
				fmt.Printf("URL: \033[34m%s\033[0m\nReflected Characters: \033[31m%v\033[0m %s\n", displayURL, reflected, basic)
			}
		}
	})

	for sc.Scan() {
		inputURL := sc.Text()
		// Ensure URL ends with a trailing slash
		if !strings.HasSuffix(inputURL, "/") {
			inputURL += "/"
		}

		finalURL := getFinalRedirectURL(inputURL)
		baseURL, path := splitBaseURLAndPath(finalURL)
		initialPathChecks <- pathCheck{
			originalURL: inputURL,
			finalURL:    finalURL,
			baseURL:     baseURL,
			path:        path,
		}
	}

	close(initialPathChecks)
	<-donePaths
}

func getFinalRedirectURL(inputURL string) string {
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequest("GET", inputURL, nil)
	if err != nil {
		return inputURL
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return inputURL
	}
	defer resp.Body.Close()

	// Follow redirects to get the final URL
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		finalURL := resp.Header.Get("Location")
		if !strings.HasPrefix(finalURL, "http") {
			finalURL = req.URL.Scheme + "://" + req.URL.Host + finalURL
		}
		// Ensure the final URL ends with a trailing slash
		if !strings.HasSuffix(finalURL, "/") {
			finalURL += "/"
		}
		return getFinalRedirectURL(finalURL)
	}

	return inputURL
}

func splitBaseURLAndPath(inputURL string) (string, string) {
	u, err := url.Parse(inputURL)
	if err != nil {
		return inputURL, ""
	}
	// Strip query parameters and fragments
	path := u.Path
	// Ensure the path ends with a trailing slash
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return u.Scheme + "://" + u.Host, path
}

func checkPathReflected(baseURL, path string) ([]string, string, error) {
	reflectedChars := map[string]bool{}
	specialChars := []string{"\"", "'", "<", ">", "$", "|", "(", ")", ":", ";", "{", "}"}
	identifier := "smhtahxssin33" // Identifier to attach directly with special characters

	identifierReflected := false
	hasSpecialCharsReflected := false

	// Check if only the identifier is reflected
	reqURLIdentifier := baseURL + path + identifier
	req, err := http.NewRequest("GET", reqURLIdentifier, nil)
	if err != nil {
		return nil, "", err
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.100 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// Reading the response body
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	body := string(b)

	// Check if the body contains only the identifier
	if strings.Contains(body, identifier) {
		identifierReflected = true
	}

	// Check if the body contains the special character after the identifier
	for _, char := range specialChars {
		reqURL := baseURL + path + identifier + char

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, "", err
		}

		req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.100 Safari/537.36")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer resp.Body.Close()

		// Reading the response body
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, "", err
		}

		body = string(b)

		// Check if the body contains the special character after the identifier
		if strings.Contains(body, identifier+char) {
			reflectedChars[char] = true
			hasSpecialCharsReflected = true
		}
	}

	// Convert map to slice to eliminate duplicates
	reflected := []string{}
	for char := range reflectedChars {
		reflected = append(reflected, char)
	}

	if identifierReflected {
		if hasSpecialCharsReflected {
			return reflected, "", nil
		}
		return nil, "[basic]", nil
	}

	return reflected, "", nil
}

func makePoolPath(input chan pathCheck, fn workerFuncPath) chan pathCheck {
	var wg sync.WaitGroup

	output := make(chan pathCheck)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range input {
				fn(c, output)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}

type workerFuncPath func(pathCheck, chan pathCheck)
