package remotehttpservice

import (
	"encoding/json"

	"github.com/QueerGlobal/hub-framework/core/entity"
)

// forwardedJSONResponse holds unmarshaled upstream JSON while preserving HTTP response metadata.
type forwardedJSONResponse struct {
	meta entity.ResponseMeta
	data map[string]interface{}
}

func (r *forwardedJSONResponse) GetResponseMeta() entity.ResponseMeta {
	return r.meta
}

func (r *forwardedJSONResponse) SetResponseMeta(meta entity.ResponseMeta) {
	r.meta = meta
}

func (r *forwardedJSONResponse) GetBody() []byte {
	body, err := json.Marshal(r.data)
	if err != nil {
		return nil
	}
	return body
}

func (r *forwardedJSONResponse) SetBody(body []byte) {
	_ = json.Unmarshal(body, &r.data)
}

func (r *forwardedJSONResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.data)
}
