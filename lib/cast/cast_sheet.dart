import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../playback_session.dart';
import '../ui/player/playback_sheets.dart';
import 'cast_controller.dart';

/// 原生侧抛回来的错误只需要那句中文，不要把异常类型糊到界面上。
String castMessage(Object failure) => failure is PlatformException
    ? (failure.message ?? '投屏失败')
    : failure.toString();

/// 打开投屏选设备面板。复用播放器的深色主题与抽屉尺寸。
Future<void> showCastSheet(
  BuildContext context, {
  required PlaybackSession session,
}) => showPlayerSheet(context, (context) => CastSheet(session: session));

String _clock(Duration value) {
  String two(int number) => number.toString().padLeft(2, '0');
  final seconds = value.inSeconds;
  final clock = '${two(seconds ~/ 60 % 60)}:${two(seconds % 60)}';
  return seconds >= 3600 ? '${seconds ~/ 3600}:$clock' : clock;
}

class CastSheet extends StatefulWidget {
  const CastSheet({super.key, required this.session});
  final PlaybackSession session;

  @override
  State<CastSheet> createState() => _CastSheetState();
}

class _CastSheetState extends State<CastSheet> {
  final _cast = CastController.instance;
  bool _busy = false;
  String? _failure;

  @override
  void initState() {
    super.initState();
    if (!_cast.casting && _cast.devices.isEmpty && !_cast.scanning) {
      unawaited(_cast.scan());
    }
  }

  Future<void> _select(CastDevice target) async {
    if (_busy) return;
    final session = widget.session;
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      // 电视拉不到手机的 127.0.0.1，地址要换成核心在局域网口上开的同一份流。
      final base = await session.repository.castBase();
      final path = Uri.parse(session.streamUrl).path;
      await _cast.start(
        target: target,
        url: '$base${path.isEmpty ? '/' : path}',
        title: session.castTitle,
        position: Duration(
          milliseconds: (session.currentPosition * 1000).round(),
        ),
      );
      // 本机暂停，免得两头同时播；片子仍由本机经局域网转发给电视。
      if (!session.paused) session.togglePlayback();
      if (!mounted) return;
      final messenger = ScaffoldMessenger.of(context);
      Navigator.of(context).pop();
      messenger.showSnackBar(SnackBar(content: Text('已投屏到 ${target.name}')));
    } catch (failure) {
      if (!mounted) return;
      setState(() {
        _failure = castMessage(failure);
        _busy = false;
      });
    }
  }

  Future<void> _stop() async {
    setState(() => _busy = true);
    await _cast.stop();
    await widget.session.repository.releaseCastBase();
    if (!mounted) return;
    setState(() => _busy = false);
  }

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: _cast,
    builder: (context, _) {
      final colors = Theme.of(context).colorScheme;
      final hint = TextStyle(fontSize: 13, color: colors.onSurfaceVariant);
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SheetHeader(
            children: [
              const Expanded(child: Text('投屏到电视', style: sheetTitle)),
              IconButton(
                onPressed: () => Navigator.pop(context),
                icon: const Icon(Icons.close_rounded),
                tooltip: '关闭',
              ),
            ],
          ),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.fromLTRB(16, 4, 16, 24),
              children: [
                if (_cast.casting) ...[
                  _connected(context, hint),
                  const SizedBox(height: 12),
                  Text('停止投屏后可以接着在本机看，进度不会同步回来。', style: hint),
                ] else ...[
                  Text(
                    '把当前这一集投到同一 Wi-Fi 下的电视 / 盒子 / 投影仪，'
                    '手机上会暂停，投屏期间请不要退出播放器。',
                    style: hint,
                  ),
                  const SizedBox(height: 12),
                  if (_cast.scanning)
                    const Padding(
                      padding: EdgeInsets.symmetric(vertical: 24),
                      child: Center(child: CircularProgressIndicator()),
                    )
                  else if (_cast.devices.isEmpty)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 16),
                      child: Text(_cast.error ?? '没有找到可投屏的设备', style: hint),
                    )
                  else
                    for (final device in _cast.devices)
                      ListTile(
                        contentPadding: EdgeInsets.zero,
                        enabled: !_busy,
                        leading: const Icon(Icons.tv_rounded),
                        title: Text(device.name),
                        subtitle: Text('DLNA · 点击投屏'),
                        onTap: _busy ? null : () => _select(device),
                      ),
                  if (_failure case final failure?) ...[
                    const SizedBox(height: 8),
                    Text(failure, style: hint.copyWith(color: colors.error)),
                  ],
                  const SizedBox(height: 16),
                  Align(
                    alignment: Alignment.centerLeft,
                    child: OutlinedButton.icon(
                      onPressed: _cast.scanning || _busy
                          ? null
                          : () => unawaited(_cast.scan()),
                      icon: const Icon(Icons.refresh_rounded),
                      label: const Text('重新搜索'),
                    ),
                  ),
                ],
                if (_busy)
                  const Padding(
                    padding: EdgeInsets.only(top: 16),
                    child: LinearProgressIndicator(),
                  ),
              ],
            ),
          ),
        ],
      );
    },
  );

  Widget _connected(BuildContext context, TextStyle hint) {
    final colors = Theme.of(context).colorScheme;
    final duration = _cast.duration;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Card(
          margin: EdgeInsets.zero,
          color: colors.surfaceContainerHighest,
          child: ListTile(
            leading: Icon(Icons.tv_rounded, color: colors.primary),
            title: Text(_cast.device?.name ?? '投屏设备'),
            subtitle: Text(
              duration > Duration.zero
                  ? '正在投放 · ${_clock(_cast.position)} / ${_clock(duration)}'
                  : '正在投放 · ${_clock(_cast.position)}',
            ),
          ),
        ),
        const SizedBox(height: 12),
        Row(
          spacing: 8,
          children: [
            FilledButton.icon(
              onPressed: _busy
                  ? null
                  : () => unawaited(
                      _cast.playing ? _cast.pause() : _cast.resume(),
                    ),
              icon: Icon(
                _cast.playing ? Icons.pause_rounded : Icons.play_arrow_rounded,
              ),
              label: Text(_cast.playing ? '暂停' : '继续'),
            ),
            OutlinedButton.icon(
              onPressed: _busy ? null : () => unawaited(_stop()),
              icon: const Icon(Icons.stop_rounded),
              label: const Text('停止投屏'),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text('暂停 / 继续只影响电视，本机画面保持暂停。', style: hint),
      ],
    );
  }
}
