package measure

import (
	"errors"
	"strconv"
	"strings"
)

var errMalformedKey = errors.New("measure: malformed group key")

func encodeKey(dst []byte, values []string) []byte {
	for _, v := range values {
		dst = strconv.AppendInt(dst, int64(len(v)), 10)
		dst = append(dst, ':')
		dst = append(dst, v...)
	}

	return dst
}

func decodeKey(key string) ([]string, error) {
	var out []string

	for i := 0; i < len(key); {
		sep := strings.IndexByte(key[i:], ':')

		if sep < 0 {
			return nil, errMalformedKey
		}

		n, err := strconv.Atoi(key[i : i+sep])

		if err != nil || n < 0 {
			return nil, errMalformedKey
		}

		i += sep + 1

		if i+n > len(key) {
			return nil, errMalformedKey
		}

		out = append(out, key[i:i+n])
		i += n
	}

	return out, nil
}
