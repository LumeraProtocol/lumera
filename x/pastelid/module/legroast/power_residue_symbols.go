package legroast

import (
	"lukechampine.com/uint128"
)

var powerResidueSymbolMap = map[uint128.Uint128]byte{
	uint128.New(1, 0): 0,
	uint128.New(18446726481523507199, 9223372036854775807): 1,
	uint128.New(0, 16777216):                               2,
	uint128.New(18446744073709551583, 9223372036854775807): 3,
	uint128.New(562949953421312, 0):                        4,
	uint128.New(18446744073709551615, 9223372036317904895): 5,
	uint128.New(1024, 0):                                   6,
	uint128.New(18428729675200069631, 9223372036854775807): 7,
	uint128.New(0, 17179869184):                            8,
	uint128.New(18446744073709518847, 9223372036854775807): 9,
	uint128.New(576460752303423488, 0):                     10,
	uint128.New(18446744073709551615, 9223371487098961919): 11,
	uint128.New(1048576, 0):                                12,
	uint128.New(18446744073709551615, 9223372036854775806): 13,
	uint128.New(0, 17592186044416):                         14,
	uint128.New(18446744073675997183, 9223372036854775807): 15,
	uint128.New(0, 32):                                     16,
	uint128.New(18446744073709551615, 9222809086901354495): 17,
	uint128.New(1073741824, 0):                             18,
	uint128.New(18446744073709551615, 9223372036854774783): 19,
	uint128.New(0, 18014398509481984):                      20,
	uint128.New(18446744039349813247, 9223372036854775807): 21,
	uint128.New(0, 32768):                                  22,
	uint128.New(18446744073709551615, 8646911284551352319): 23,
	uint128.New(1099511627776, 0):                          24,
	uint128.New(18446744073709551615, 9223372036853727231): 25,
	uint128.New(2, 0):                                      26,
	uint128.New(18446708889337462783, 9223372036854775807): 27,
	uint128.New(0, 33554432):                               28,
	uint128.New(18446744073709551551, 9223372036854775807): 29,
	uint128.New(1125899906842624, 0):                       30,
	uint128.New(18446744073709551615, 9223372035781033983): 31,
	uint128.New(2048, 0):                                   32,
	uint128.New(18410715276690587647, 9223372036854775807): 33,
	uint128.New(0, 34359738368):                            34,
	uint128.New(18446744073709486079, 9223372036854775807): 35,
	uint128.New(1152921504606846976, 0):                    36,
	uint128.New(18446744073709551615, 9223370937343148031): 37,
	uint128.New(2097152, 0):                                38,
	uint128.New(18446744073709551615, 9223372036854775805): 39,
	uint128.New(0, 35184372088832):                         40,
	uint128.New(18446744073642442751, 9223372036854775807): 41,
	uint128.New(0, 64):                                     42,
	uint128.New(18446744073709551615, 9222246136947933183): 43,
	uint128.New(2147483648, 0):                             44,
	uint128.New(18446744073709551615, 9223372036854773759): 45,
	uint128.New(0, 36028797018963968):                      46,
	uint128.New(18446744004990074879, 9223372036854775807): 47,
	uint128.New(0, 65536):                                  48,
	uint128.New(18446744073709551615, 8070450532247928831): 49,
	uint128.New(2199023255552, 0):                          50,
	uint128.New(18446744073709551615, 9223372036852678655): 51,
	uint128.New(4, 0):                                      52,
	uint128.New(18446673704965373951, 9223372036854775807): 53,
	uint128.New(0, 67108864):                               54,
	uint128.New(18446744073709551487, 9223372036854775807): 55,
	uint128.New(2251799813685248, 0):                       56,
	uint128.New(18446744073709551615, 9223372034707292159): 57,
	uint128.New(4096, 0):                                   58,
	uint128.New(18374686479671623679, 9223372036854775807): 59,
	uint128.New(0, 68719476736):                            60,
	uint128.New(18446744073709420543, 9223372036854775807): 61,
	uint128.New(2305843009213693952, 0):                    62,
	uint128.New(18446744073709551615, 9223369837831520255): 63,
	uint128.New(4194304, 0):                                64,
	uint128.New(18446744073709551615, 9223372036854775803): 65,
	uint128.New(0, 70368744177664):                         66,
	uint128.New(18446744073575333887, 9223372036854775807): 67,
	uint128.New(0, 128):                                    68,
	uint128.New(18446744073709551615, 9221120237041090559): 69,
	uint128.New(4294967296, 0):                             70,
	uint128.New(18446744073709551615, 9223372036854771711): 71,
	uint128.New(0, 72057594037927936):                      72,
	uint128.New(18446743936270598143, 9223372036854775807): 73,
	uint128.New(0, 131072):                                 74,
	uint128.New(18446744073709551615, 6917529027641081855): 75,
	uint128.New(4398046511104, 0):                          76,
	uint128.New(18446744073709551615, 9223372036850581503): 77,
	uint128.New(8, 0):                                      78,
	uint128.New(18446603336221196287, 9223372036854775807): 79,
	uint128.New(0, 134217728):                              80,
	uint128.New(18446744073709551359, 9223372036854775807): 81,
	uint128.New(4503599627370496, 0):                       82,
	uint128.New(18446744073709551615, 9223372032559808511): 83,
	uint128.New(8192, 0):                                   84,
	uint128.New(18302628885633695743, 9223372036854775807): 85,
	uint128.New(0, 137438953472):                           86,
	uint128.New(18446744073709289471, 9223372036854775807): 87,
	uint128.New(4611686018427387904, 0):                    88,
	uint128.New(18446744073709551615, 9223367638808264703): 89,
	uint128.New(8388608, 0):                                90,
	uint128.New(18446744073709551615, 9223372036854775799): 91,
	uint128.New(0, 140737488355328):                        92,
	uint128.New(18446744073441116159, 9223372036854775807): 93,
	uint128.New(0, 256):                                    94,
	uint128.New(18446744073709551615, 9218868437227405311): 95,
	uint128.New(8589934592, 0):                             96,
	uint128.New(18446744073709551615, 9223372036854767615): 97,
	uint128.New(0, 144115188075855872):                     98,
	uint128.New(18446743798831644671, 9223372036854775807): 99,
	uint128.New(0, 262144):                                 100,
	uint128.New(18446744073709551615, 4611686018427387903): 101,
	uint128.New(8796093022208, 0):                          102,
	uint128.New(18446744073709551615, 9223372036846387199): 103,
	uint128.New(16, 0):                                     104,
	uint128.New(18446462598732840959, 9223372036854775807): 105,
	uint128.New(0, 268435456):                              106,
	uint128.New(18446744073709551103, 9223372036854775807): 107,
	uint128.New(9007199254740992, 0):                       108,
	uint128.New(18446744073709551615, 9223372028264841215): 109,
	uint128.New(16384, 0):                                  110,
	uint128.New(18158513697557839871, 9223372036854775807): 111,
	uint128.New(0, 274877906944):                           112,
	uint128.New(18446744073709027327, 9223372036854775807): 113,
	uint128.New(9223372036854775808, 0):                    114,
	uint128.New(18446744073709551615, 9223363240761753599): 115,
	uint128.New(16777216, 0):                               116,
	uint128.New(18446744073709551615, 9223372036854775791): 117,
	uint128.New(0, 281474976710656):                        118,
	uint128.New(18446744073172680703, 9223372036854775807): 119,
	uint128.New(0, 512):                                    120,
	uint128.New(18446744073709551615, 9214364837600034815): 121,
	uint128.New(17179869184, 0):                            122,
	uint128.New(18446744073709551615, 9223372036854759423): 123,
	uint128.New(0, 288230376151711744):                     124,
	uint128.New(18446743523953737727, 9223372036854775807): 125,
	uint128.New(0, 524288):                                 126,
	uint128.New(18446744073709551614, 9223372036854775807): 127,
	uint128.New(17592186044416, 0):                         128,
	uint128.New(18446744073709551615, 9223372036837998591): 129,
	uint128.New(32, 0):                                     130,
	uint128.New(18446181123756130303, 9223372036854775807): 131,
	uint128.New(0, 536870912):                              132,
	uint128.New(18446744073709550591, 9223372036854775807): 133,
	uint128.New(18014398509481984, 0):                      134,
	uint128.New(18446744073709551615, 9223372019674906623): 135,
	uint128.New(32768, 0):                                  136,
	uint128.New(17870283321406128127, 9223372036854775807): 137,
	uint128.New(0, 549755813888):                           138,
	uint128.New(18446744073708503039, 9223372036854775807): 139,
	uint128.New(0, 1):                                      140,
	uint128.New(18446744073709551615, 9223354444668731391): 141,
	uint128.New(33554432, 0):                               142,
	uint128.New(18446744073709551615, 9223372036854775775): 143,
	uint128.New(0, 562949953421312):                        144,
	uint128.New(18446744072635809791, 9223372036854775807): 145,
	uint128.New(0, 1024):                                   146,
	uint128.New(18446744073709551615, 9205357638345293823): 147,
	uint128.New(34359738368, 0):                            148,
	uint128.New(18446744073709551615, 9223372036854743039): 149,
	uint128.New(0, 576460752303423488):                     150,
	uint128.New(18446742974197923839, 9223372036854775807): 151,
	uint128.New(0, 1048576):                                152,
	uint128.New(18446744073709551613, 9223372036854775807): 153,
	uint128.New(35184372088832, 0):                         154,
	uint128.New(18446744073709551615, 9223372036821221375): 155,
	uint128.New(64, 0):                                     156,
	uint128.New(18445618173802708991, 9223372036854775807): 157,
	uint128.New(0, 1073741824):                             158,
	uint128.New(18446744073709549567, 9223372036854775807): 159,
	uint128.New(36028797018963968, 0):                      160,
	uint128.New(18446744073709551615, 9223372002495037439): 161,
	uint128.New(65536, 0):                                  162,
	uint128.New(17293822569102704639, 9223372036854775807): 163,
	uint128.New(0, 1099511627776):                          164,
	uint128.New(18446744073707454463, 9223372036854775807): 165,
	uint128.New(0, 2):                                      166,
	uint128.New(18446744073709551615, 9223336852482686975): 167,
	uint128.New(67108864, 0):                               168,
	uint128.New(18446744073709551615, 9223372036854775743): 169,
	uint128.New(0, 1125899906842624):                       170,
	uint128.New(18446744071562067967, 9223372036854775807): 171,
	uint128.New(0, 2048):                                   172,
	uint128.New(18446744073709551615, 9187343239835811839): 173,
	uint128.New(68719476736, 0):                            174,
	uint128.New(18446744073709551615, 9223372036854710271): 175,
	uint128.New(0, 1152921504606846976):                    176,
	uint128.New(18446741874686296063, 9223372036854775807): 177,
	uint128.New(0, 2097152):                                178,
	uint128.New(18446744073709551611, 9223372036854775807): 179,
	uint128.New(70368744177664, 0):                         180,
	uint128.New(18446744073709551615, 9223372036787666943): 181,
	uint128.New(128, 0):                                    182,
	uint128.New(18444492273895866367, 9223372036854775807): 183,
	uint128.New(0, 2147483648):                             184,
	uint128.New(18446744073709547519, 9223372036854775807): 185,
	uint128.New(72057594037927936, 0):                      186,
	uint128.New(18446744073709551615, 9223371968135299071): 187,
	uint128.New(131072, 0):                                 188,
	uint128.New(16140901064495857663, 9223372036854775807): 189,
	uint128.New(0, 2199023255552):                          190,
	uint128.New(18446744073705357311, 9223372036854775807): 191,
	uint128.New(0, 4):                                      192,
	uint128.New(18446744073709551615, 9223301668110598143): 193,
	uint128.New(134217728, 0):                              194,
	uint128.New(18446744073709551615, 9223372036854775679): 195,
	uint128.New(0, 2251799813685248):                       196,
	uint128.New(18446744069414584319, 9223372036854775807): 197,
	uint128.New(0, 4096):                                   198,
	uint128.New(18446744073709551615, 9151314442816847871): 199,
	uint128.New(137438953472, 0):                           200,
	uint128.New(18446744073709551615, 9223372036854644735): 201,
	uint128.New(0, 2305843009213693952):                    202,
	uint128.New(18446739675663040511, 9223372036854775807): 203,
	uint128.New(0, 4194304):                                204,
	uint128.New(18446744073709551607, 9223372036854775807): 205,
	uint128.New(140737488355328, 0):                        206,
	uint128.New(18446744073709551615, 9223372036720558079): 207,
	uint128.New(256, 0):                                    208,
	uint128.New(18442240474082181119, 9223372036854775807): 209,
	uint128.New(0, 4294967296):                             210,
	uint128.New(18446744073709543423, 9223372036854775807): 211,
	uint128.New(144115188075855872, 0):                     212,
	uint128.New(18446744073709551615, 9223371899415822335): 213,
	uint128.New(262144, 0):                                 214,
	uint128.New(13835058055282163711, 9223372036854775807): 215,
	uint128.New(0, 4398046511104):                          216,
	uint128.New(18446744073701163007, 9223372036854775807): 217,
	uint128.New(0, 8):                                      218,
	uint128.New(18446744073709551615, 9223231299366420479): 219,
	uint128.New(268435456, 0):                              220,
	uint128.New(18446744073709551615, 9223372036854775551): 221,
	uint128.New(0, 4503599627370496):                       222,
	uint128.New(18446744065119617023, 9223372036854775807): 223,
	uint128.New(0, 8192):                                   224,
	uint128.New(18446744073709551615, 9079256848778919935): 225,
	uint128.New(274877906944, 0):                           226,
	uint128.New(18446744073709551615, 9223372036854513663): 227,
	uint128.New(0, 4611686018427387904):                    228,
	uint128.New(18446735277616529407, 9223372036854775807): 229,
	uint128.New(0, 8388608):                                230,
	uint128.New(18446744073709551599, 9223372036854775807): 231,
	uint128.New(281474976710656, 0):                        232,
	uint128.New(18446744073709551615, 9223372036586340351): 233,
	uint128.New(512, 0):                                    234,
	uint128.New(18437736874454810623, 9223372036854775807): 235,
	uint128.New(0, 8589934592):                             236,
	uint128.New(18446744073709535231, 9223372036854775807): 237,
	uint128.New(288230376151711744, 0):                     238,
	uint128.New(18446744073709551615, 9223371761976868863): 239,
	uint128.New(524288, 0):                                 240,
	uint128.New(9223372036854775807, 9223372036854775807):  241,
	uint128.New(0, 8796093022208):                          242,
	uint128.New(18446744073692774399, 9223372036854775807): 243,
	uint128.New(0, 16):                                     244,
	uint128.New(18446744073709551615, 9223090561878065151): 245,
	uint128.New(536870912, 0):                              246,
	uint128.New(18446744073709551615, 9223372036854775295): 247,
	uint128.New(0, 9007199254740992):                       248,
	uint128.New(18446744056529682431, 9223372036854775807): 249,
	uint128.New(0, 16384):                                  250,
	uint128.New(18446744073709551615, 8935141660703064063): 251,
	uint128.New(549755813888, 0):                           252,
	uint128.New(18446744073709551615, 9223372036854251519): 253,
}

// powerResidueSymbol calculates the power residue symbol for the given input.
func powerResidueSymbol(a *uint128.Uint128) byte {
	out := *a
	temp := uint128.Uint128{}

	for i := 0; i < 17; i++ {
		// Square 7 times and multiply by 'a'
		squareModP(&temp, &out)
		squareModP(&out, &temp)
		squareModP(&temp, &out)
		squareModP(&out, &temp)
		squareModP(&temp, &out)
		squareModP(&out, &temp)
		squareModP(&temp, &out)
		out = uint128.Uint128{}
		mulAddModP(&out, &temp, a)
	}

	reduceModP(&out)

	// Check if the power residue symbol is in the map
	if symbol, exists := powerResidueSymbolMap[out]; exists {
		return symbol
	} else {
		return 0
	}
}
