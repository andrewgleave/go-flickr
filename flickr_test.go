package flickr

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func TestGetInfo(t *testing.T) {
	r := &Request{
		APIKey: "YOURAPIKEYHERE",
		Method: "flickr.photos.getInfo",
		Args: map[string]string{
			"photo_id": "5356343650",
		},
	}

	// Don't need to sign but might as well since we're testing
	r.Sign("YOURAPISECRETHERE")

	fmt.Println(r.URL())

	res, err := r.Execute()
	if err != nil {
		fmt.Println(err)
		return
	}

	defer res.Close()
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(body)
}
