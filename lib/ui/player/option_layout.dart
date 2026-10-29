/// 播放器弹出菜单（清晰度 / 倍速 / 播放线路）的选项分列排布。
///
/// 线路多的源一集能有二三十条线路，而弹出菜单贴着控制条向上展开、本身不会
/// 滚动，单列排下来高度会超过窗口，顶部的几项直接被裁掉、后面的也点不到。
/// 这里按「每列最多几行」把一维选项切成若干列：先从上往下读，读完一列再排
/// 下一列，和菜单的阅读习惯一致。
///
/// 线路再多的源（几十上百条）连多列也排不下，这时由 [planOptionGrid] 把列数
/// 钉死在可用宽度上、改成按行填充，再交给弹层套一层竖向滚动兜底。
library;

/// 把 [values] 按每列最多 [maxRows] 行切成若干列。
///
/// [maxRows] 小于 1 时退回成单行一列（不做切分）；选项本身为空时返回空列表。
List<List<T>> optionColumns<T>(List<T> values, {required int maxRows}) {
  if (values.isEmpty) {
    return <List<T>>[];
  }
  final rows = maxRows < 1 || maxRows > values.length ? values.length : maxRows;
  final count = (values.length / rows).ceil();
  return <List<T>>[
    for (var column = 0; column < count; column += 1)
      <T>[
        for (var row = 0; row < rows; row += 1)
          if (column * rows + row < values.length) values[column * rows + row],
      ],
  ];
}

/// 把 [values] 按行填成 [columns] 列的若干行，最后一行可以不满。
///
/// 和 [optionColumns] 的阅读顺序相反：这里从左往右读，一行读完再读下一行，
/// 配合竖向滚动时视线不会在列之间来回跳。
List<List<T>> optionRows<T>(List<T> values, {required int columns}) {
  if (values.isEmpty || columns < 1) {
    return <List<T>>[];
  }
  return <List<T>>[
    for (var start = 0; start < values.length; start += columns)
      values.sublist(start, (start + columns).clamp(0, values.length)),
  ];
}

/// 弹层选项网格的排版规划。
class OptionGridPlan {
  const OptionGridPlan({
    required this.columns,
    required this.rows,
    required this.perColumn,
    required this.scrolls,
    required this.height,
  });

  /// 网格总列数。
  final int columns;

  /// 网格总行数。
  final int rows;

  /// 是否按列填充（整屏放得下时先竖着读完一列再读下一列）。
  final bool perColumn;

  /// 是否超出可用高度，需要竖向滚动才能看全。
  final bool scrolls;

  /// 网格实际高度。
  final double height;

  /// 只剩一列时不必填充方向，直接竖排即可。
  bool get singleColumn => columns <= 1;
}

/// 按可用空间算弹层该排几列几行，以及放不下时要不要滚动。
///
/// 优先「整屏放下」：行数由可用高度决定，列数由行数反推；列数一旦超过可用
/// 宽度能容纳的列数，就把列数钉在可容纳的最大值、改成按行填充，把多出来的
/// 行交给竖向滚动，这样无论线路有多少条都不会被裁掉。
OptionGridPlan planOptionGrid({
  required int count,
  required double availableHeight,
  required double availableWidth,
  required double rowHeight,
  required double columnWidth,
  required double gap,
  int minRows = 2,
  int maxRows = 10,
}) {
  if (count <= 0 || rowHeight <= 0 || columnWidth <= 0 || gap < 0) {
    return const OptionGridPlan(
      columns: 0,
      rows: 0,
      perColumn: true,
      scrolls: false,
      height: 0,
    );
  }
  final heightRows = (availableHeight / rowHeight).floor();
  final fitCols = ((availableWidth + gap) / (columnWidth + gap)).floor();
  final maxCols = fitCols.clamp(1, count);
  final rowCap = heightRows.clamp(minRows, maxRows).clamp(1, count);
  final columns = (count / rowCap).ceil();
  if (columns <= maxCols) {
    return OptionGridPlan(
      columns: columns,
      rows: rowCap,
      perColumn: true,
      scrolls: false,
      height: rowCap * rowHeight,
    );
  }
  final rows = (count / maxCols).ceil();
  return OptionGridPlan(
    columns: maxCols,
    rows: rows,
    perColumn: false,
    scrolls: rows > heightRows,
    height: rows * rowHeight,
  );
}
