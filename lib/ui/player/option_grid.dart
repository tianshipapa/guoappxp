import 'package:flutter/material.dart';

import 'option_layout.dart';

/// 播放器弹层选项网格的公共尺寸。
abstract final class OptionGridMetrics {
  /// 单个选项的高度。
  static const double rowHeight = 36;

  /// 单列时选项的最小宽度（文字自己撑开）。
  static const double minWidth = 72;

  /// 多列时每个格子的固定宽度：格宽固定才能精确算出总宽、判断要不要滚动。
  static const double cellWidth = 96;

  /// 列与列之间的间距。
  static const double gap = 10;

  /// 滚动条让出来的宽度，免得压在最后一列上。
  static const double scrollGutter = 8;
}

/// 播放器弹层（清晰度 / 倍速 / 播放线路）的选项网格。
///
/// 弹层是贴在控制条上方的 `Positioned`，拿到的约束是**无界**的（0~∞），所以
/// 这里任何一层的宽度都必须自己算出来：单列靠 `IntrinsicWidth` 撑开，多列每个
/// 格子钉死 [OptionGridMetrics.cellWidth]，滚动分支再用 `ConstrainedBox` 兜住。
///
/// 注意别在这里用 `CrossAxisAlignment.stretch`：Column 的 stretch 会把子项宽度
/// 约束成 `constraints.maxWidth`，在无界约束下就是无穷大，整块弹层会静默地
/// 排不出来（release 下连断言都没有，表现是「点了没反应」）。
class OptionGrid<T> extends StatelessWidget {
  const OptionGrid({
    super.key,
    required this.plan,
    required this.availableHeight,
    required this.values,
    required this.selected,
    required this.label,
    required this.onSelected,
    required this.scrollController,
  });

  /// 由 [planOptionGrid] 算好的排版。
  final OptionGridPlan plan;

  /// 弹层能用的高度，滚动分支按它截断。
  final double availableHeight;

  final List<T> values;
  final T selected;
  final String Function(T) label;
  final void Function(T) onSelected;

  /// 选项太多要滚动时用的控制器（滚动条也要它才能定位）。
  final ScrollController scrollController;

  /// 网格实际占用的宽度（含滚动条让位），弹层据此判断要不要横向挪位。
  static double widthOf(OptionGridPlan plan) {
    final bare =
        plan.columns * OptionGridMetrics.cellWidth +
        (plan.columns > 1 ? (plan.columns - 1) * OptionGridMetrics.gap : 0.0);
    return plan.scrolls ? bare + OptionGridMetrics.scrollGutter : bare;
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    Widget cell(T value, double? width) => InkWell(
      borderRadius: BorderRadius.circular(8),
      hoverColor: Colors.white.withValues(alpha: .14),
      onTap: () => onSelected(value),
      child: Container(
        height: OptionGridMetrics.rowHeight,
        width: width,
        constraints: BoxConstraints(
          minWidth: width ?? OptionGridMetrics.minWidth,
        ),
        padding: const EdgeInsets.symmetric(horizontal: 14),
        alignment: Alignment.center,
        child: Text(
          label(value),
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(
            fontSize: 14,
            fontWeight: value == selected ? FontWeight.w700 : FontWeight.w500,
            color: value == selected ? colors.primary : Colors.white,
          ),
        ),
      ),
    );

    if (plan.singleColumn) {
      // IntrinsicWidth 给出一份有限的紧约束，Column 才能安全地按内容撑开。
      return IntrinsicWidth(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [for (final value in values) cell(value, null)],
        ),
      );
    }

    if (plan.perColumn) {
      // 一列一列往下读：整屏放得下时就是这个排法。列宽用 SizedBox 钉死，
      // 不依赖 stretch（见类注释）。
      final columns = optionColumns(values, maxRows: plan.rows);
      return Row(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          for (final (index, column) in columns.indexed) ...[
            if (index > 0) const SizedBox(width: OptionGridMetrics.gap),
            SizedBox(
              width: OptionGridMetrics.cellWidth,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  for (final value in column)
                    cell(value, OptionGridMetrics.cellWidth),
                ],
              ),
            ),
          ],
        ],
      );
    }

    // 放不下整屏：按行填充，多出来的行交给竖向滚动。
    final grid = Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        for (final row in optionRows(values, columns: plan.columns))
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              for (final (index, value) in row.indexed) ...[
                if (index > 0) const SizedBox(width: OptionGridMetrics.gap),
                cell(value, OptionGridMetrics.cellWidth),
              ],
            ],
          ),
      ],
    );
    if (!plan.scrolls) {
      return grid;
    }
    final width = widthOf(plan);
    return ConstrainedBox(
      constraints: BoxConstraints(maxWidth: width, maxHeight: availableHeight),
      child: ScrollbarTheme(
        // 弹层底色是恒定的深色，滚动条得跟着用浅色，否则亮主题下几乎看不见。
        data: const ScrollbarThemeData(
          thumbColor: WidgetStatePropertyAll<Color?>(Colors.white54),
        ),
        child: Scrollbar(
          controller: scrollController,
          thumbVisibility: true,
          thickness: 4,
          radius: const Radius.circular(2),
          child: SingleChildScrollView(
            controller: scrollController,
            padding: const EdgeInsets.only(
              right: OptionGridMetrics.scrollGutter,
            ),
            child: grid,
          ),
        ),
      ),
    );
  }
}
