import 'package:duanju_app/ui/player/option_layout.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('选项不超过每列行数时保持单列', () {
    expect(optionColumns([1, 2, 3], maxRows: 5), [
      [1, 2, 3],
    ]);
  });

  test('超过每列行数时往下读满一列再排下一列', () {
    expect(optionColumns([1, 2, 3, 4, 5, 6, 7], maxRows: 3), [
      [1, 2, 3],
      [4, 5, 6],
      [7],
    ]);
  });

  test('刚好整除时不留空列', () {
    expect(optionColumns([1, 2, 3, 4, 5, 6], maxRows: 3), [
      [1, 2, 3],
      [4, 5, 6],
    ]);
  });

  test('行数限制非法时退回整列显示，不丢选项', () {
    expect(optionColumns([1, 2, 3], maxRows: 0), [
      [1, 2, 3],
    ]);
    expect(optionColumns([1, 2, 3], maxRows: -1), [
      [1, 2, 3],
    ]);
  });

  test('空列表不产生空列', () {
    expect(optionColumns<int>([], maxRows: 4), isEmpty);
  });

  test('切分后顺序与总数不变（线路序号不能错位）', () {
    final values = [for (var route = 0; route < 23; route += 1) route];
    final columns = optionColumns(values, maxRows: 8);
    expect(columns.length, 3);
    expect(columns.expand((column) => column).toList(), values);
    expect(columns.every((column) => column.length <= 8), isTrue);
  });
}
