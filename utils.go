package tuihub

import "github.com/invopop/jsonschema"

func ReflectJSONSchema(v interface{}) (string, error) {
	r := new(jsonschema.Reflector)
	r.ExpandedStruct = true
	r.DoNotReference = true
	j, err := r.Reflect(v).MarshalJSON()
	if err != nil {
		return "", err
	}
	return string(j), nil
}

func MustReflectJSONSchema(v interface{}) string {
	j, err := ReflectJSONSchema(v)
	if err != nil {
		panic(err)
	}
	return j
}
