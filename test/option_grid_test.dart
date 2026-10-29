import 'package:duanju_app/ui/player/option_grid.dart';
import 'package:duanju_app/ui/player/option_layout.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// 弹层是贴在控制条上方的 `Positioned`（只给 left/top），拿到的约束是无界的
/// 0~∞。这里按同样的方式挂载，任何「宽度依赖外部约束」的写法都会在这里露馅，
/// 例如 Column 的 stretch 会把子项撑成无限宽、整块静默排不出来。
Widget _popup(Widget child) => MaterialApp(
  home: Scaffold(
    backgroundColor: Colors.black,
    body: Stack(
      fit: StackFit.expand,
      children: [
        Positioned(
          left: 0,
          top: 0,
          child: Material(
            color: const Color(0xE0141414),
            borderRadius: BorderRadius.circular(12),
            child: Padding(padding: const EdgeInsets.all(6), child: child),
          ),
        ),
      ],
    ),
  ),
);

OptionGrid<int> _grid(
  List<int> values, {
  required OptionGridPlan plan,
  required double availableHeight,
  required ScrollController controller,
  void Function(int)? onSelected,
}) => OptionGrid<int>(
  plan: plan,
  availableHeight: availableHeight,
  values: values,
  selected: 0,
  label: (value) => '线路 ${value + 1}',
  onSelected: onSelected ?? (_) {},
  scrollController: controller,
);

OptionGridPlan _plan(int count, {double height = 288, double width = 308}) =>
    planOptionGrid(
      count: count,
      availableHeight: height,
      availableWidth: width,
      rowHeight: OptionGridMetrics.rowHeight,
      columnWidth: OptionGridMetrics.cellWidth,
      gap: OptionGridMetrics.gap,
    );

void main() {
  testWidgets('单列弹层在无界约束下按内容撑开', (tester) async {
    final controller = ScrollController();
    addTearDown(controller.dispose);
    final values = [0, 1, 2];
    await tester.pumpWidget(
      _popup(
        _grid(
          values,
          plan: _plan(values.length),
          availableHeight: 288,
          controller: controller,
        ),
      ),
    );
    expect(tester.takeException(), isNull);
    expect(find.text('线路 1'), findsOneWidget);
    expect(find.text('线路 3'), findsOneWidget);
    final size = tester.getSize(find.byType(OptionGrid<int>));
    expect(size.height, 3 * OptionGridMetrics.rowHeight);
    expect(size.width, greaterThan(0));
    expect(size.width, lessThanOrEqualTo(OptionGridMetrics.cellWidth + 32));
  });

  testWidgets('线路分列时不会把格子撑成无限宽（Windows 上曾整块排不出来）', (tester) async {
    final controller = ScrollController();
    addTearDown(controller.dispose);
    final values = [for (var route = 0; route < 23; route += 1) route];
    final plan = _plan(values.length);
    expect(plan.perColumn, isTrue);
    expect(plan.columns, 3);
    await tester.pumpWidget(
      _popup(
        _grid(values, plan: plan, availableHeight: 288, controller: controller),
      ),
    );
    expect(tester.takeException(), isNull);
    final size = tester.getSize(find.byType(OptionGrid<int>));
    expect(size.width, OptionGrid.widthOf(plan));
    expect(
      size.width,
      3 * OptionGridMetrics.cellWidth + 2 * OptionGridMetrics.gap,
    );
    expect(size.height, plan.rows * OptionGridMetrics.rowHeight);
    // 每一条都在，而且最右边那一列真的在网格之内（不是被挤到无穷远处）。
    for (final value in values) {
      expect(find.text('线路 ${value + 1}'), findsOneWidget);
    }
    final last = tester.getRect(find.text('线路 23'));
    expect(last.right, lessThanOrEqualTo(size.width + 1));
  });

  testWidgets('分列时点选仍然能命中对应的线路', (tester) async {
    final controller = ScrollController();
    addTearDown(controller.dispose);
    final values = [for (var route = 0; route < 23; route += 1) route];
    final picked = <int>[];
    final plan = _plan(values.length);
    await tester.pumpWidget(
      _popup(
        _grid(
          values,
          plan: plan,
          availableHeight: 288,
          controller: controller,
          onSelected: picked.add,
        ),
      ),
    );
    await tester.tap(find.text('线路 12'));
    expect(picked, [11]);
  });

  testWidgets('线路特别多时按行填充并出现竖向滚动条', (tester) async {
    final controller = ScrollController();
    addTearDown(controller.dispose);
    final values = [for (var route = 0; route < 60; route += 1) route];
    final plan = _plan(values.length);
    expect(plan.scrolls, isTrue);
    await tester.pumpWidget(
      _popup(
        _grid(values, plan: plan, availableHeight: 288, controller: controller),
      ),
    );
    expect(tester.takeException(), isNull);
    expect(find.byType(Scrollbar), findsOneWidget);
    final size = tester.getSize(find.byType(OptionGrid<int>));
    expect(size.width, OptionGrid.widthOf(plan));
    expect(size.height, 288);
    expect(controller.position.maxScrollExtent, greaterThan(0));
    await tester.drag(find.text('线路 1'), const Offset(0, -200));
    await tester.pumpAndSettle();
    expect(controller.offset, greaterThan(0));
    expect(tester.takeException(), isNull);
  });
}
