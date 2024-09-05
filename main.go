package main

import (
	"bufio"
	"crypto/tls"
	"flag"
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

var (
	debug       bool
	concurrency int
	transport   = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: time.Second,
			DualStack: true,
		}).DialContext,
	}
	httpClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // Adjust timeout as needed
	}
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Enable debug output")
	flag.IntVar(&concurrency, "t", 40, "Number of concurrent workers") // Set default concurrency to 40
	flag.Parse()
}

func main() {
	sc := bufio.NewScanner(os.Stdin)
	hasInput := false
	initialPathChecks := make(chan pathCheck, 100)
	donePaths := makePoolPath(initialPathChecks, func(c pathCheck, output chan pathCheck) {
		// Perform path reflection check on the final redirected URL
		reflected, basic, err := checkPathReflected(c.baseURL, c.path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error from checkPathReflected for URL %s with path %s: %s\n", c.baseURL, c.path, err)
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
		hasInput = true
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

	if !hasInput {
		fmt.Fprintln(os.Stderr, "Error: No input provided.")
		return
	}

	close(initialPathChecks)
	<-donePaths
}

func getFinalRedirectURL(inputURL string) string {
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Limit the number of redirects
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		return nil
	}

	req, err := http.NewRequest("GET", inputURL, nil)
	if err != nil {
		if debug {
			fmt.Printf("\033[31mError creating request for URL %s: %s\033[0m\n", inputURL, err)
		}
		return inputURL
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if debug {
			fmt.Printf("\033[31mError making request for URL %s: %s\033[0m\n", inputURL, err)
		}
		return inputURL
	}
	defer resp.Body.Close()

	// Check for a Location header to identify redirects
	if location := resp.Header.Get("Location"); location != "" {
		if !strings.HasPrefix(location, "http") {
			location = req.URL.Scheme + "://" + req.URL.Host + location
		}
		if !strings.HasSuffix(location, "/") {
			location += "/"
		}
		return getFinalRedirectURL(location)
	}

	// If no Location header is present, return the current URL
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
	} else if debug {
		fmt.Printf("\033[33mNo reflection for identifier %s in URL %s\033[0m\n", identifier, reqURLIdentifier)
	}

	if identifierReflected {
		hasSpecialCharsReflected := false
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

		if hasSpecialCharsReflected {
			return reflected, "", nil
		}
		return nil, "[basic]", nil
	}

	return nil, "", nil
}

func makePoolPath(input chan pathCheck, fn workerFuncPath) chan pathCheck {
	var wg sync.WaitGroup

	output := make(chan pathCheck)
	for i := 0; i < concurrency; i++ { // Use the concurrency flag
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
