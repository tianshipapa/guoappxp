// 生成网页端的拼音首字母表：native/server/web/pinyin.js
//
// 本地 Web 服务版是手写的 JS 界面，浏览器里没有拼音库，所以要把
// 「汉字 → 拼音首字母」的对照表随包发出去，收藏夹 / 播放记录才能像 App 那样
// 用 BDZC 搜到《霸道总裁》。表按码位平铺：从 U+4E00 起，一个码位一个字符，
// 取不到首字母（生僻字、符号、非汉字）写 '_'，前端据此跳过。
//
// 用法（改了 lpinyin 版本或想换码位区间时重跑）：
//   dart run tool/gen_pinyin_table.dart
//
// 表由 lpinyin 生成，和 App 侧 catalog_sort.dart 的 matchesTitle 用的是同一份
// 拼音数据，两端结果不会打架。
import 'dart:io';

import 'package:lpinyin/lpinyin.dart';

const start = 0x4e00; // 一
const end = 0x9fff; // 龿，覆盖常用汉字与 GBK 全区间

void main() {
  final table = StringBuffer();
  var resolved = 0;
  for (var code = start; code <= end; code += 1) {
    final char = String.fromCharCode(code);
    var initial = '_';
    try {
      final short = PinyinHelper.getShortPinyin(char).toLowerCase();
      if (short.length == 1 &&
          short.codeUnitAt(0) >= 0x61 &&
          short.codeUnitAt(0) <= 0x7a) {
        initial = short;
        resolved += 1;
      }
    } on PinyinException {
      // 词典里没有这个字，留 '_'，前端直接忽略。
    }
    table.write(initial);
  }

  final file = File('native/server/web/pinyin.js');
  file.writeAsStringSync(
    '// 由 tool/gen_pinyin_table.dart 生成，勿手改。\n'
    '// 从 U+${start.toRadixString(16).toUpperCase()} 起，一个码位一位拼音首字母，'
    "'_' 表示取不到。\n"
    'window.DUANJU_PINYIN_INITIALS =\n'
    "  '${table.toString()}';\n",
  );
  stdout.writeln(
    '已生成 ${file.path}：${end - start + 1} 个码位，'
    '$resolved 个有首字母，${file.lengthSync()} 字节',
  );
}
