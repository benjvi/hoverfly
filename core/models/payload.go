package models

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"encoding/base64"
	"encoding/gob"
	"io"
	"regexp"
	log "github.com/Sirupsen/logrus"
	"net/http"
	"strings"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/json"
	"github.com/tdewolff/minify/xml"
	"github.com/SpectoLabs/hoverfly/core/views"
)

const (
	contentTypeJSON = "application/json"
	contentTypeXML = "application/xml"
	otherType = "otherType"
)

var (
	rxJSON = regexp.MustCompile("[/+]json$")
	rxXML = regexp.MustCompile("[/+]xml$")
	// mime types which will not be base 64 encoded when exporting as JSON
	supportedMimeTypes = [...]string{"text", "plain", "css", "html", "json", "xml", "js", "javascript"}
	minifiers * minify.M
)

func init() {
	// GetNewMinifiers - sets minify.M with prepared xml/json minifiers
	minifiers = minify.New()
	minifiers.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)
	minifiers.AddFuncRegexp(regexp.MustCompile("[/+]json$"), json.Minify)
}

// Payload structure holds request and response structure
type Payload struct {
	Response ResponseDetails `json:"response"`
	Request  RequestDetails  `json:"request"`
}

func (p Payload) Id() string {
	return p.Request.Hash()
}

func (p Payload) IdWithoutHost() string {
	return p.Request.HashWithoutHost()
}

// Encode method encodes all exported Payload fields to bytes
func (p *Payload) Encode() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(p)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (p *Payload) ConvertToPayloadView() (*views.PayloadView) {
	return &views.PayloadView{Response: p.Response.ConvertToResponseDetailsView(), Request: p.Request.ConvertToRequestDetailsView()}
}

// NewPayloadFromBytes decodes supplied bytes into Payload structure
func NewPayloadFromBytes(data []byte) (*Payload, error) {
	var p *Payload
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(&p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func NewPayloadFromPayloadView(data views.PayloadView) (Payload) {
	return Payload{
		Response: NewResponseDetialsFromResponseDetailsView(data.Response),
		Request: NewRequestDetailsFromRequestDetailsView(data.Request),
	}
}

// RequestDetails stores information about request, it's used for creating unique hash and also as a payload structure
type RequestDetails struct {
	Path        string              `json:"path"`
	Method      string              `json:"method"`
	Destination string              `json:"destination"`
	Scheme      string              `json:"scheme"`
	Query       string              `json:"query"`
	Body        string              `json:"body"`
	Headers     map[string][]string `json:"headers"`
}

func NewRequestDetailsFromRequestDetailsView(data views.RequestDetailsView) (RequestDetails) {
	return RequestDetails{
		Path: data.Path,
		Method: data.Method,
		Destination: data.Destination,
		Scheme: data.Scheme,
		Query: data.Query,
		Body: data.Body,
		Headers: data.Headers,
	}
}

func (r *RequestDetails) ConvertToRequestDetailsView() (views.RequestDetailsView) {
	return views.RequestDetailsView{
		Path: r.Path,
		Method: r.Method,
		Destination: r.Destination,
		Scheme: r.Scheme,
		Query: r.Query,
		Body: r.Body,
		Headers: r.Headers,
	}
}

func (r *RequestDetails) concatenate(withHost bool) string {
	var buffer bytes.Buffer

	if withHost {
		buffer.WriteString(r.Destination)
	}

	buffer.WriteString(r.Path)
	buffer.WriteString(r.Method)
	buffer.WriteString(r.Query)
	if len(r.Body) > 0 {
		ct := r.getContentType()

		if ct == contentTypeJSON || ct == contentTypeXML {
			buffer.WriteString(r.minifyBody(ct))
		} else {
			log.WithFields(log.Fields{
				"content-type": r.Headers["Content-Type"],
			}).Debug("unknown content type")

			buffer.WriteString(r.Body)
		}
	}

	return buffer.String()
}

func (r *RequestDetails) minifyBody(mediaType string) (minified string) {
	var err error
	minified, err = minifiers.String(mediaType, r.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"error":       err.Error(),
			"destination": r.Destination,
			"path":        r.Path,
			"method":      r.Method,
		}).Errorf("failed to minify request body, media type given: %s. Request matching might fail", mediaType)
		return r.Body
	}
	log.Debugf("body minified, mediatype: %s", mediaType)
	return minified
}

func (r *RequestDetails) Hash() string {
	h := md5.New()
	io.WriteString(h, r.concatenate(true))
	return fmt.Sprintf("%x", h.Sum(nil))
}
func (r *RequestDetails) HashWithoutHost() string {
	h := md5.New()
	io.WriteString(h, r.concatenate(false))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (r *RequestDetails) getContentType() string {
	for _, v := range r.Headers["Content-Type"] {
		if rxJSON.MatchString(v) {
			return contentTypeJSON
		}
		if rxXML.MatchString(v) {
			return contentTypeXML
		}
	}
	return otherType
}

// ResponseDetails structure hold response body from external service, body is not decoded and is supposed
// to be bytes, however headers should provide all required information for later decoding
// by the client.
type ResponseDetails struct {
	Status  int                 `json:"status"`
	Body    string              `json:"body"`
	Headers map[string][]string `json:"headers"`
}

func NewResponseDetialsFromResponseDetailsView(data views.ResponseDetailsView) (ResponseDetails) {
	body := data.Body

	if data.EncodedBody == true {
		decoded, _ := base64.StdEncoding.DecodeString(data.Body)
		body = string(decoded)
	}

	return ResponseDetails{Status: data.Status, Body: body, Headers: data.Headers}
}



func (r *ResponseDetails) ConvertToResponseDetailsView() (views.ResponseDetailsView) {
	needsEncoding := false

	// Check headers for gzip
	contentEncodingValues := r.Headers["Content-Encoding"]
	if len(contentEncodingValues) > 0 {
		needsEncoding = true
	} else {
		mimeType := http.DetectContentType([]byte(r.Body))
		needsEncoding = true
		for _, v := range supportedMimeTypes {
			if strings.Contains(mimeType, v) {
				needsEncoding = false
				break
			}
		}
	}

	// If contains gzip, base64 encode
	body := r.Body
	if (needsEncoding) {
		body = base64.StdEncoding.EncodeToString([]byte(r.Body))
	}

	return views.ResponseDetailsView{Status: r.Status, Body: body, Headers: r.Headers, EncodedBody: needsEncoding}
}