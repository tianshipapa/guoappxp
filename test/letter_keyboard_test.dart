import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:duanju_app/ui/app_scope.dart';
import 'package:duanju_app/ui/widgets/letter_keyboard.dart';

import 'support/fakes.dart';

/// 字母键盘的按键用了 FocusRing，FocusRing 要从 AppScope 读电视模式，
/// 所以测试里必须带上一个 AppScope（仓库用测试替身即可）。
Widget _host(Widget home) =>
    AppScope(repository: FakeRepository(), child: MaterialApp(home: home));

void main() {
  testWidgets('字母键盘按拼音首字母实时列出匹配并可选中', (tester) async {
    String? picked;
    const titles = ['霸道总裁', '白日梦我', '长安十二时辰'];
    Future<List<String>> search(String query) async => [
      for (final title in titles)
        if (_initials(title).contains(query.toUpperCase())) title,
    ];

    await tester.pumpWidget(
      _host(
        Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: TextButton(
                onPressed: () async {
                  picked = await showLetterKeyboard(
                    context,
                    text: '',
                    matches: search,
                  );
                },
                child: const Text('打开'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('打开'));
    await tester.pumpAndSettle();
    expect(find.text('输入剧名拼音首字母，例如 BDZC'), findsOneWidget);

    // B + D：只有「霸道总裁」（BDZC）能命中，候选项来自 debounce 之后的结果。
    await tester.tap(find.text('B'));
    await tester.pump();
    await tester.tap(find.text('D'));
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pumpAndSettle();
    expect(find.text('霸道总裁'), findsOneWidget);
    expect(find.text('白日梦我'), findsNothing);

    await tester.tap(find.text('霸道总裁'));
    await tester.pumpAndSettle();
    expect(picked, '霸道总裁');
  });

  testWidgets('退格与清空能改回上一次输入', (tester) async {
    await tester.pumpWidget(
      _host(
        Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: TextButton(
                onPressed: () => showLetterKeyboard(context, text: '霸道'),
                child: const Text('打开'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('打开'));
    await tester.pumpAndSettle();
    // 带初始文本进入：显示的是原文，不是占位提示。
    expect(find.text('霸道'), findsOneWidget);

    await tester.tap(find.byIcon(Icons.backspace_outlined));
    await tester.pumpAndSettle();
    expect(find.text('霸'), findsOneWidget);

    await tester.tap(find.text('清空'));
    await tester.pumpAndSettle();
    expect(find.text('输入剧名拼音首字母，例如 BDZC'), findsOneWidget);
  });
}

/// 测试里不依赖 lpinyin，直接给出这几部剧的首字母，保持断言稳定。
String _initials(String title) => switch (title) {
  '霸道总裁' => 'BDZC',
  '白日梦我' => 'BRMW',
  '长安十二时辰' => 'CASESC',
  _ => '',
};
