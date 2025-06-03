package proxy

import (
	"bytes"
	"encoding/json"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// PrettyJSONEncoder is an ecoder to pretty print json outputs
type PrettyJSONEncoder struct {
	zapcore.Encoder
}

func (e *PrettyJSONEncoder) Clone() zapcore.Encoder {
	return &PrettyJSONEncoder{
		Encoder: e.Encoder.Clone(),
	}
}

func (e *PrettyJSONEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	buf, err := e.Encoder.EncodeEntry(ent, fields)
	if err != nil {
		return nil, err
	}

	// do pretty printing
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, buf.Bytes(), "", "  "); err != nil {
		return nil, err
	}

	newBuf := buffer.NewPool().Get()
	newBuf.AppendString(prettyJSON.String() + "\n")
	return newBuf, nil
}
