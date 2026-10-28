/// 播放器弹出菜单（清晰度 / 倍速 / 播放线路）的选项分列排布。
///
/// 线路多的源一集能有二三十条线路，而弹出菜单贴着控制条向上展开、本身不会
/// 滚动，单列排下来高度会超过窗口，顶部的几项直接被裁掉、后面的也点不到。
/// 这里按「每列最多几行」把一维选项切成若干列：先从上往下读，读完一列再排
/// 下一列，和菜单的阅读习惯一致。
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
