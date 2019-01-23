package muxcodec

import (
	mc "mbfs/go-mbfs/gx/QmYMiyZRYDmhMr2phMc4FGrYbsyzvR751BgeobnWroiq2z/go-multicodec"
	cbor "mbfs/go-mbfs/gx/QmYMiyZRYDmhMr2phMc4FGrYbsyzvR751BgeobnWroiq2z/go-multicodec/cbor"
	json "mbfs/go-mbfs/gx/QmYMiyZRYDmhMr2phMc4FGrYbsyzvR751BgeobnWroiq2z/go-multicodec/json"
)

func StandardMux() *Multicodec {
	return MuxMulticodec([]mc.Multicodec{
		cbor.Multicodec(),
		json.Multicodec(false),
		json.Multicodec(true),
	}, SelectFirst)
}
