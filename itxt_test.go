package pngitxt

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
	"time"
)

func TestPNGiTXt(t *testing.T) {
	f, err := ioutil.ReadFile("testdata/1.png")
	if err != nil {
		t.Fatalf("Open(): %s", err)
	}

	gotRes, err := NewPNGiTXt(bytes.NewReader(f))
	if err != nil {
		t.Fatal(err)
	}
	for i, i2 := range gotRes.GetAll() {
		fmt.Println(i, "----", string(i2))
	}
	gotRes.Set("time", []byte(time.Now().Format(time.RFC3339)))

	b := &bytes.Buffer{}
	err = gotRes.Write(b)
	if err != nil {
		t.Fatal(err)
	}
	err = ioutil.WriteFile("testdata/1.png", b.Bytes(), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
