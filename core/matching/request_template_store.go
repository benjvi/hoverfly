package matching

import (
	"github.com/SpectoLabs/hoverfly/core/models"
	"errors"
	"net/http"
	"reflect"
	log "github.com/Sirupsen/logrus"
	"fmt"
	"encoding/json"
	"github.com/SpectoLabs/hoverfly/core/views"
)

type RequestTemplateStore []RequestTemplatePayload


type RequestTemplatePayload struct {
	RequestTemplate RequestTemplate        `json:"requestTemplate"`
	Response        models.ResponseDetails `json:"response"`
}

type RequestTemplatePayloadView struct {
	RequestTemplate RequestTemplate        `json:"requestTemplate"`
	Response        views.ResponseDetailsView `json:"response"`
}

type RequestTemplatePayloadJson struct {
	Data *[]RequestTemplatePayloadView `json:"data"`
}

type RequestTemplate struct {
	Path        *string              `json:"path"`
	Method      *string              `json:"method"`
	Destination *string              `json:"destination"`
	Scheme      *string              `json:"scheme"`
	Query       *string              `json:"query"`
	Body        *string              `json:"body"`
	Headers     map[string][]string `json:"headers"`
}

func(this *RequestTemplateStore) GetPayload(req *http.Request, reqBody []byte, webserver bool) (*models.Payload, error) {
	// iterate through the request templates, looking for template to match request
	for _, entry := range *this {
		// TODO: not matching by default on URL and body - need to enable this
		// TODO: need to enable regex matches
		// TODO: enable matching on scheme

		if entry.RequestTemplate.Body != nil && *entry.RequestTemplate.Body == string(reqBody) {
			continue
		}
		if (!webserver) {
			if entry.RequestTemplate.Destination != nil && *entry.RequestTemplate.Destination != req.Host {
				continue
			}
		}
		if entry.RequestTemplate.Path != nil && *entry.RequestTemplate.Path != req.URL.Path {
			continue
		}
		if entry.RequestTemplate.Query != nil && *entry.RequestTemplate.Query != req.URL.RawQuery {
			continue
		}
		if !headerMatch(entry.RequestTemplate.Headers, req.Header) {
			continue
		}
		if entry.RequestTemplate.Method != nil && *entry.RequestTemplate.Method != req.Method {
			continue
		}

		// return the first template to match
		return &models.Payload{Response: entry.Response}, nil
	}
	return nil, errors.New("No match found")
}

// ImportPayloads - a function to save given payloads into the database.
func (this *RequestTemplateStore) ImportPayloads(payloadsView RequestTemplatePayloadJson) error {
	if len(*payloadsView.Data) > 0 {
		// Convert PayloadView back to Payload for internal storage
		payloads := payloadsView.ConvertToRequestTemplateStore()
		for _, pl := range payloads {

			//TODO: add hooks for concsistency with request import
			// note that importing hoverfly is a disallowed circular import

			*this = append(*this, pl)
		}
		log.WithFields(log.Fields{
			"total":      len(*this),
		}).Info("payloads imported")
		return nil
	}
	return fmt.Errorf("Bad request. Nothing to import!")
}

func (this *RequestTemplateStore) Wipe() {
	// don't change the pointer here!
	*this = RequestTemplateStore{}
}

/**
Check keys and corresponding values in template headers are also present in request headers
 */
func headerMatch(tmplHeaders map[string][]string, reqHeaders http.Header) (bool) {

	for headerName, headerVal := range tmplHeaders {
		// TODO: case insensitive lookup
		// TODO: is order of values in slice really important?

		reqHeaderVal, ok := reqHeaders[headerName]
		if (ok && reflect.DeepEqual(headerVal, reqHeaderVal)) {
			continue;
		} else {
			return false;
		}
	}
	return true;
}

func(this *RequestTemplateStore) ConvertToPayloadJson() (RequestTemplatePayloadJson) {
	var payloadViewList []RequestTemplatePayloadView
	for _, v := range *this {
		payloadViewList = append(payloadViewList, v.ConvertToRequestTemplatePayloadView())
	}
	return RequestTemplatePayloadJson{
		Data: &payloadViewList,
	}
}

func(this *RequestTemplatePayload) ConvertToRequestTemplatePayloadView() (RequestTemplatePayloadView) {
	return RequestTemplatePayloadView{
		RequestTemplate: this.RequestTemplate,
		Response: this.Response.ConvertToResponseDetailsView(),
	}
}

func(this *RequestTemplatePayloadJson) ConvertToRequestTemplateStore() (RequestTemplateStore) {
	var requestTemplateStore RequestTemplateStore
	for _, v := range *this.Data {
		requestTemplateStore = append(requestTemplateStore, v.ConvertToPayload())
	}
	return requestTemplateStore
}

func(this *RequestTemplatePayloadView) ConvertToPayload() (RequestTemplatePayload) {
	return RequestTemplatePayload{
		RequestTemplate: this.RequestTemplate,
		Response: models.NewResponseDetialsFromResponseDetailsView(this.Response),
	}
}

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil

}
