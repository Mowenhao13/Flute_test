package oti

type Oti struct {
	FECEncodingID uint8
	FECInstanceID uint16 
	MaximumSourceBlockLength uint16 
	EncodingSymbolLength uint16 
}


func NewNoCode(encodingSymbolLength uint16) Oti {
	return Oti{
		FECEncodingID: 0,
		FECInstanceID: 0,
		EncodingSymbolLength:    encodingSymbolLength,
		MaximumSourceBlockLength: 0,
	}
}

func NewRaptorQ(encodingSymbolLength uint16) Oti {
	return Oti{
		FECEncodingID: 1,
		FECInstanceID: 1,
		EncodingSymbolLength:    encodingSymbolLength,
		MaximumSourceBlockLength: 0,
	}	
}