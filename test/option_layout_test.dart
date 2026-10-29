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

  test('按行填充时从左往右读，最后一行可以不满', () {
    expect(optionRows([1, 2, 3, 4, 5, 6, 7], columns: 3), [
      [1, 2, 3],
      [4, 5, 6],
      [7],
    ]);
    expect(optionRows([1, 2], columns: 5), [
      [1, 2],
    ]);
    expect(optionRows<int>([], columns: 3), isEmpty);
    expect(optionRows([1, 2], columns: 0), isEmpty);
  });

  OptionGridPlan plan(
    int count, {
    double height = 300,
    double width = 400,
  }) => planOptionGrid(
    count: count,
    availableHeight: height,
    availableWidth: width,
    rowHeight: 36,
    columnWidth: 96,
    gap: 10,
  );

  test('选项少时单列显示，不需要滚动', () {
    final result = plan(3);
    expect(result.columns, 1);
    expect(result.rows, 3);
    expect(result.perColumn, isTrue);
    expect(result.scrolls, isFalse);
    expect(result.singleColumn, isTrue);
  });

  test('放得下时按高度分列，先读满一列再读下一列', () {
    // 300 高放得下 8 行，400 宽放得下 3 列。
    final result = plan(23);
    expect(result.perColumn, isTrue);
    expect(result.scrolls, isFalse);
    expect(result.rows, 8);
    expect(result.columns, 3);
    expect(result.height, 8 * 36);
  });

  test('列数超过宽度能放下的列数时改成按行填充并滚动', () {
    // 60 条：按 8 行分列要 8 列，但只放得下 3 列，于是 3 列 20 行 + 竖向滚动。
    final result = plan(60);
    expect(result.perColumn, isFalse);
    expect(result.scrolls, isTrue);
    expect(result.columns, 3);
    expect(result.rows, 20);
    expect(result.height, 20 * 36);
  });

  test('窗口够高时哪怕列数被压也不滚动', () {
    // 1000 高能放 27 行（上限 10 行），600 宽只放得下 5 列。
    final result = plan(61, height: 1000, width: 600);
    expect(result.perColumn, isFalse);
    expect(result.scrolls, isFalse);
    expect(result.columns, 5);
    expect(result.rows, 13);
  });

  test('空间极小或选项为空时给出安全结果，不抛异常', () {
    final empty = plan(0);
    expect(empty.columns, 0);
    expect(empty.rows, 0);
    expect(empty.singleColumn, isTrue);

    final tiny = plan(50, height: 10, width: 10);
    expect(tiny.columns, 1);
    expect(tiny.scrolls, isTrue);
    expect(tiny.rows, 50);
  });
}
