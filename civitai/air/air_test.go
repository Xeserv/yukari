package air

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		air    string
		result ID
	}{
		{
			air: "urn:air:sd1:model:civitai:2421@43533",
			result: ID{
				Ecosystem:    "sd1",
				Type:         "model",
				Source:       "civitai",
				ModelID:      "2421",
				ModelVersion: "43533",
			},
		},
		{
			air: "urn:air:sd2:model:civitai:2421@43533",
			result: ID{
				Ecosystem:    "sd2",
				Type:         "model",
				Source:       "civitai",
				ModelID:      "2421",
				ModelVersion: "43533",
			},
		},
		{
			air: "urn:air:sdxl:lora:civitai:328553@368189",
			result: ID{
				Ecosystem:    "sdxl",
				Type:         "lora",
				Source:       "civitai",
				ModelID:      "328553",
				ModelVersion: "368189",
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.air, func(t *testing.T) {
			id, err := Parse(cs.air)
			if err != nil {
				t.Fatalf("%s is not valid: %v", cs.air, err)
			}

			if *id != cs.result {
				t.Fatalf("air %s is not the same as the input", cs.result.String())
			}

			if id.String() != cs.air {
				t.Fatalf("air %s doesn't stringify as %s", cs.air, id.String())
			}
		})
	}
}
