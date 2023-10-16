package holoinsightlogsextension

import (
	"bytes"
	"encoding/json"
	"go.uber.org/multierr"
	"io"
	"net/http"
	"sync"
)

type Data struct {
	Logs   []map[string]string `mapstructure:"__logs__" json:"__logs__"`
	Tags   map[string]string   `mapstructure:"__tags__" json:"__tags__"`
	Topic  string              `mapstructure:"__topic__" json:"__topic__"`
	Source string              `mapstructure:"__source__" json:"__source__"`
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func getBuffer() *bytes.Buffer {
	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

func putBuffer(buffer *bytes.Buffer) {
	bufferPool.Put(buffer)
}

func handlePayload(req *http.Request) (log *Data, err error) {
	defer func() {
		_, errs := io.Copy(io.Discard, req.Body)
		err = multierr.Combine(err, errs, req.Body.Close())
	}()

	buf := getBuffer()
	defer putBuffer(buf)
	if _, err = io.Copy(buf, req.Body); err != nil {
		return nil, err
	}
	var data *Data
	err = json.Unmarshal(buf.Bytes(), &data)
	return data, err
}
