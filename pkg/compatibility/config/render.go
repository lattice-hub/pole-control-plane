// Package config 只在显式启用的外部协议适配器中转换统一配置快照。
package config

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/magiconair/properties"
	"golang.org/x/net/html"
	"gopkg.in/yaml.v3"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
)

const MaxBytes = 512 * 1024

var (
	namePattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*)*$`)
	integerPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]*[1-9])?$`)
	errRender      = errors.New("configuration rendering failed")
)

// Render 校验完整输入和输出上界后渲染；literal 恒等复制任意字节，错误不含配置内容。
func Render(ctx context.Context, snapshot *configv1.RenderSnapshot) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if snapshot == nil || len(snapshot.Content) > MaxBytes {
		return nil, errRender
	}
	if snapshot.Engine == "literal" {
		if len(snapshot.Parameters)+len(snapshot.Values) != 0 {
			return nil, errRender
		}
		return bytes.Clone(snapshot.Content), nil
	}
	if snapshot.Engine != "pole-mustache-v1" || !utf8.Valid(snapshot.Content) || len(snapshot.Parameters) > 128 || len(snapshot.Values) > 128 {
		return nil, errRender
	}
	declared := make(map[string]*configv1.Parameter, len(snapshot.Parameters))
	total := len(snapshot.Content)
	for _, parameter := range snapshot.Parameters {
		if parameter == nil || !namePattern.MatchString(parameter.Name) || declared[parameter.Name] != nil || parameter.Type < configv1.ValueType_VALUE_TYPE_STRING || parameter.Type > configv1.ValueType_VALUE_TYPE_DECIMAL {
			return nil, errRender
		}
		declared[parameter.Name] = parameter
		total += len(parameter.Name)
		if parameter.Required && snapshot.Values[parameter.Name] == nil {
			return nil, errRender
		}
	}
	for name, value := range snapshot.Values {
		parameter := declared[name]
		if parameter == nil || value == nil || parameter.Type != value.Type || !validScalar(value) {
			return nil, errRender
		}
		total += len(value.Value)
	}
	if total > MaxBytes {
		return nil, errRender
	}
	// 先解析并累计最终长度，再分配；重复大变量不会造成分配膨胀。
	template := string(snapshot.Content)
	type part struct{ text string }
	parts := make([]part, 0)
	size := 0
	for len(template) > 0 {
		open, close := strings.Index(template, "{{"), strings.Index(template, "}}")
		if close >= 0 && (open < 0 || close < open) {
			return nil, errRender
		}
		if open < 0 {
			parts = append(parts, part{template})
			size += len(template)
			break
		}
		parts = append(parts, part{template[:open]})
		size += open
		template = template[open:]
		end := strings.Index(template, "}}}")
		if !strings.HasPrefix(template, "{{{") || end < 3 {
			return nil, errRender
		}
		name := template[3:end]
		if !namePattern.MatchString(name) || declared[name] == nil || snapshot.Values[name] == nil {
			return nil, errRender
		}
		value := snapshot.Values[name].Value
		size += len(value)
		if size > MaxBytes {
			return nil, errRender
		}
		parts = append(parts, part{value})
		template = template[end+3:]
	}
	if size > MaxBytes {
		return nil, errRender
	}
	result := make([]byte, 0, size)
	for _, part := range parts {
		result = append(result, part.text...)
	}
	if err := validateFormat(snapshot.Format, result); err != nil {
		return nil, errRender
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func validScalar(value *configv1.Scalar) bool {
	switch value.Type {
	case configv1.ValueType_VALUE_TYPE_STRING:
		return utf8.ValidString(value.Value)
	case configv1.ValueType_VALUE_TYPE_BOOLEAN:
		return value.Value == "true" || value.Value == "false"
	case configv1.ValueType_VALUE_TYPE_INTEGER:
		return value.Value != "-0" && integerPattern.MatchString(value.Value)
	case configv1.ValueType_VALUE_TYPE_DECIMAL:
		return value.Value != "-0" && decimalPattern.MatchString(value.Value)
	default:
		return false
	}
}

func validateFormat(format string, content []byte) error {
	switch strings.ToLower(format) {
	case "", "text":
		return nil
	case "json":
		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.UseNumber()
		if err := jsonValue(decoder); err != nil {
			return err
		}
		if _, err := decoder.Token(); err != io.EOF {
			return errRender
		}
		return nil
	case "yaml", "yml":
		var document any
		return yaml.Unmarshal(content, &document)
	case "xml":
		return validateXML(content)
	case "properties":
		// properties的${...}属于应用字面值，不是第二套模板语言。
		// 库的续行器只消费一个换行字符；仅解析副本使用LF，输出仍保留原始CRLF。
		_, err := (&properties.Loader{Encoding: properties.UTF8, DisableExpansion: true}).LoadBytes(bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n")))
		return err
	case "html":
		_, err := html.Parse(bytes.NewReader(content))
		return err
	default:
		return errRender
	}
}

// jsonValue 保留数字精度并拒绝任意层级重复 key；标准 json.Valid 不拒绝重复 key。
func jsonValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errRender
			}
			seen[name] = true
			if err := jsonValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := jsonValue(decoder); err != nil {
				return err
			}
		}
	default:
		return errRender
	}
	_, err = decoder.Token()
	return err
}

func validateXML(content []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	depth, roots := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(typed)) != "" {
				return errRender
			}
		}
	}
	if roots != 1 || depth != 0 {
		return errRender
	}
	return nil
}
