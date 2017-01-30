package flickr

import (
	"bytes"
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
)

const (
	endpoint        = "https://api.flickr.com/services/rest/?"
	uploadEndpoint  = "https://api.flickr.com/services/upload/"
	replaceEndpoint = "https://api.flickr.com/services/replace/"
	apiHost         = "api.flickr.com"
)

type Request struct {
	APIKey string
	Method string
	Args   map[string]string
}

type Response struct {
	Status  string         `xml:"stat,attr"`
	Error   *ResponseError `xml:"err"`
	Payload string         `xml:",innerxml"`
}

type ResponseError struct {
	Code    string `xml:"code,attr"`
	Message string `xml:"msg,attr"`
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

type Error string

func (e Error) Error() string {
	return string(e)
}

func (request *Request) Sign(secret string) {
	args := request.Args

	// Remove api_sig
	delete(args, "api_sig")

	sortedKeys := make([]string, len(args)+2)

	args["api_key"] = request.APIKey
	args["method"] = request.Method

	// Sort array keys
	i := 0
	for k := range args {
		sortedKeys[i] = k
		i++
	}
	sort.Strings(sortedKeys)

	// Build out ordered key-value string prefixed by secret
	s := secret
	for _, key := range sortedKeys {
		if args[key] != "" {
			s += fmt.Sprintf("%s%s", key, args[key])
		}
	}

	// Since we're only adding two keys, it's easier
	// and more space-efficient to just delete them
	// them copy the whole map
	delete(args, "api_key")
	delete(args, "method")

	// Have the full string, now hash
	hash := md5.New()
	hash.Write([]byte(s))

	// Add api_sig as one of the args
	args["api_sig"] = fmt.Sprintf("%x", hash.Sum(nil))
}

func (request *Request) URL() string {
	args := request.Args

	args["api_key"] = request.APIKey
	args["method"] = request.Method

	s := endpoint + encodeQuery(args)
	return s
}

func (request *Request) Execute() (*http.Response, error) {
	if request.APIKey == "" || request.Method == "" {
		return nil, Error("Need both API key and method")
	}

	s := request.URL()
	return http.Get(s)
}

func encodeQuery(args map[string]string) string {
	i := 0
	s := bytes.NewBuffer(nil)
	for k, v := range args {
		if i != 0 {
			s.WriteString("&")
		}
		i++
		s.WriteString(k + "=" + url.QueryEscape(v))
	}
	return s.String()
}

func (request *Request) buildPost(u string, filename string, filetype string) (*http.Request, error) {
	realURL, _ := url.Parse(u)

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	formSize := stat.Size()

	request.Args["api_key"] = request.APIKey

	boundary, end := "----###---###--flickr-go-rules", "\r\n"

	// Build out all of POST body sans file
	header := bytes.NewBuffer(nil)
	for k, v := range request.Args {
		header.WriteString("--" + boundary + end)
		header.WriteString("Content-Disposition: form-data; name=\"" + k + "\"" + end + end)
		header.WriteString(v + end)
	}
	header.WriteString("--" + boundary + end)
	header.WriteString("Content-Disposition: form-data; name=\"photo\"; filename=\"photo.jpg\"" + end)
	header.WriteString("Content-Type: " + filetype + end + end)

	footer := bytes.NewBufferString(end + "--" + boundary + "--" + end)

	bodyLen := int64(header.Len()) + int64(footer.Len()) + formSize

	r, w := io.Pipe()
	go func() {
		pieces := []io.Reader{header, f, footer}

		for _, k := range pieces {
			_, err = io.Copy(w, k)
			if err != nil {
				w.CloseWithError(nil)
				return
			}
		}
		f.Close()
		w.Close()
	}()

	httpHeader := make(http.Header)
	httpHeader.Add("Content-Type", "multipart/form-data; boundary="+boundary)

	postRequest := &http.Request{
		Method:        "POST",
		URL:           realURL,
		Host:          apiHost,
		Header:        httpHeader,
		Body:          r,
		ContentLength: bodyLen,
	}
	return postRequest, nil
}

// Example:
// r.Upload("thumb.jpg", "image/jpeg")
func (request *Request) Upload(filename string, filetype string) (response *Response, err error) {
	postRequest, err := request.buildPost(uploadEndpoint, filename, filetype)
	if err != nil {
		return nil, err
	}
	return sendPost(postRequest)
}

func (request *Request) Replace(filename string, filetype string) (response *Response, err error) {
	postRequest, err := request.buildPost(replaceEndpoint, filename, filetype)
	if err != nil {
		return nil, err
	}
	return sendPost(postRequest)
}

func sendPost(postRequest *http.Request) (response *Response, err error) {
	// Create and use TCP connection (lifted mostly wholesale from http.send)
	client := http.DefaultClient
	resp, err := client.Do(postRequest)

	if err != nil {
		return nil, err
	}
	rawBody, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	var r Response
	err = xml.Unmarshal(rawBody, &r)

	return &r, err
}
