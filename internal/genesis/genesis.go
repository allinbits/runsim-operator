package genesis

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	iio "github.com/allinbits/runsim-operator/internal/io"
)

func GetChainIdAndHashFromRemote(url string) (string, string, error) {
	client := http.Client{Timeout: time.Minute}
	r, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer r.Body.Close()

	pr, pw := io.Pipe()
	defer pw.Close()

	tee := iio.TeeReader(r.Body, pw)

	outCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(outCh)
		defer close(errCh)
		defer pr.Close()

		dec := json.NewDecoder(pr)

		// read open bracket
		t, err := dec.Token()
		if err != nil {
			errCh <- fmt.Errorf("error getting initial token: %v", err)
			return
		}

		// Ensure json starts with right delimiter
		if d, ok := t.(json.Delim); !ok || d != '{' {
			errCh <- fmt.Errorf("invalid json: starts with %q", d)
			return
		}

		for dec.More() {

			t, err := dec.Token()
			if err != nil {
				errCh <- fmt.Errorf("error getting token: %v", err)
				return
			}

			prop, ok := t.(string)
			if !ok {
				errCh <- fmt.Errorf("invalid property type: %T", t)
				return
			}

			if prop == "chain_id" {
				t, err = dec.Token()
				if err != nil {
					errCh <- fmt.Errorf("error token for property %q: %v", prop, err)
					return
				}

				v, ok := t.(string)
				if !ok {
					errCh <- fmt.Errorf("invalid type for property %q", prop)
					return
				}
				outCh <- v
				return
			}

			var v interface{}
			if err := dec.Decode(&v); err != nil {
				errCh <- fmt.Errorf("error decoding value for property %q: %v", prop, err)
				return
			}
		}

		errCh <- fmt.Errorf("chain_id not found")
	}()

	h := sha256.New()
	if _, err := io.Copy(h, tee); err != nil {
		return "", "", fmt.Errorf("error calculating sha256: %v", err)
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))

	if err := <-errCh; err != nil {
		return "", "", err
	}
	chainID := <-outCh

	return chainID, hash, nil
}
