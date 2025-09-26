package linker

import (
	obj "koltrakak/my-linker/myObjectFormat"
)

func Link(inputFileNames []string) (*obj.MyObjectFormat, error) {
	var inputObjs []*obj.MyObjectFormat
	for _, f := range inputFileNames {
		o, err := obj.ParseObjectFile(f)
		if err != nil {
			return nil, err
		}
		inputObjs = append(inputObjs, o)
	}

	outputObj, err := allocateStorage(inputObjs)
	if err != nil {
		return nil, err
	}

	return outputObj, nil
}

func allocateStorage(inputObjs []*obj.MyObjectFormat) (*obj.MyObjectFormat, error) {
	return nil, nil
}
