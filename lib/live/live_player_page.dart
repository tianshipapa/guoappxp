import 'dart:async';

import 'package:flutter/material.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../ui/widgets/focus_ring.dart';
import 'live_models.dart';
import 'live_playback.dart';
import 'live_repository.dart';
import 'live_sources.dart';
import 'live_store.dart';

/// 直播全屏播放页。
///
/// 与点播播放器分开实现：直播没有进度、集数与清晰度概念，控制项只有换台、
/// 换线路、暂停与收藏，避免把点播那一套状态机套到直播流上。
class LivePlayerPage extends StatefulWidget {
  const LivePlayerPage({
    super.key,
    required this.repository,
    required this.store,
    required this.source,
    required this.channels,
    this.index = 0,
  });
  final LiveRepository repository;
  final LiveStore store;
  final LiveSource source;
  final List<LiveChannel> channels;
  final int index;

  @override
  State<LivePlayerPage> createState() => _LivePlayerPageState();
}

class _LivePlayerPageState extends State<LivePlayerPage> {
  late final LivePlaybackController _playback;
  Timer? _hide;
  var _visible = true;

  @override
  void initState() {
    super.initState();
    _playback = LivePlaybackController(repository: widget.repository)
      ..addListener(_changed);
    unawaited(
      _playback.load(widget.source, widget.channels, index: widget.index),
    );
    _scheduleHide();
  }

  @override
  void dispose() {
    _hide?.cancel();
    _playback.removeListener(_changed);
    _playback.dispose();
    super.dispose();
  }

  void _changed() {
    if (mounted) setState(() {});
  }

  void _scheduleHide() {
    _hide?.cancel();
    _hide = Timer(const Duration(seconds: 4), () {
      if (mounted && _visible) setState(() => _visible = false);
    });
  }

  void _reveal() {
    setState(() => _visible = true);
    _scheduleHide();
  }

  void _toggleControls() {
    if (_visible) {
      _hide?.cancel();
      setState(() => _visible = false);
    } else {
      _reveal();
    }
  }

  LiveChannel? get _channel => _playback.channel;

  @override
  Widget build(BuildContext context) {
    final channel = _channel;
    return Scaffold(
      backgroundColor: Colors.black,
      body: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: _toggleControls,
        child: Stack(
          fit: StackFit.expand,
          children: [
            Center(child: _video()),
            if (_playback.loading)
              const Center(
                child: SizedBox(
                  width: 40,
                  height: 40,
                  child: CircularProgressIndicator(color: Colors.white),
                ),
              ),
            if (_playback.error case final error? when !_playback.loading)
              Center(child: _errorPanel(error)),
            AnimatedOpacity(
              opacity: _visible ? 1 : 0,
              duration: const Duration(milliseconds: 180),
              child: IgnorePointer(
                ignoring: !_visible,
                child: _controls(channel),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _video() {
    final controller = _playback.controller;
    if (controller == null) return const SizedBox.shrink();
    return AspectRatio(
      aspectRatio: _playback.aspectRatio,
      child: Video(
        controller: controller,
        controls: NoVideoControls,
        pauseUponEnteringBackgroundMode: false,
      ),
    );
  }

  Widget _errorPanel(String error) {
    return Container(
      width: 420,
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: Colors.black.withValues(alpha: .72),
        borderRadius: BorderRadius.circular(18),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.wifi_off_rounded, color: Colors.white70, size: 36),
          const SizedBox(height: 10),
          const Text(
            '直播中断',
            style: TextStyle(
              color: Colors.white,
              fontSize: 18,
              fontWeight: FontWeight.w700,
            ),
          ),
          const SizedBox(height: 6),
          Text(
            error,
            textAlign: TextAlign.center,
            maxLines: 3,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(color: Colors.white70, fontSize: 13),
          ),
          const SizedBox(height: 16),
          FilledButton.tonal(
            onPressed: () => unawaited(_playback.retry()),
            child: const Text('重试'),
          ),
        ],
      ),
    );
  }

  Widget _controls(LiveChannel? channel) {
    final favourite = channel != null && widget.store.isFavourite(channel.key);
    final multi = _playback.routeCount > 1;
    return Stack(
      fit: StackFit.expand,
      children: [
        const _Shade(top: true),
        const _Shade(top: false),
        Positioned(
          left: 16,
          right: 16,
          top: 12,
          child: Row(
            spacing: 8,
            children: [
              _round(
                Icons.arrow_back_rounded,
                '返回',
                () => Navigator.of(context).maybePop(),
              ),
              const SizedBox(width: 4),
              Expanded(
                child: Text(
                  channel?.label ?? '直播',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 20,
                    fontWeight: FontWeight.w700,
                    shadows: [Shadow(color: Color(0x66000000), blurRadius: 3)],
                  ),
                ),
              ),
              _round(
                favourite ? Icons.star_rounded : Icons.star_border_rounded,
                favourite ? '取消收藏' : '收藏',
                channel == null
                    ? null
                    : () => unawaited(widget.store.toggleFavourite(channel)),
              ),
            ],
          ),
        ),
        Positioned(
          left: 16,
          right: 16,
          bottom: 16,
          child: Row(
            spacing: 10,
            children: [
              _round(
                Icons.skip_previous_rounded,
                '上一台',
                _playback.channels.length < 2
                    ? null
                    : () {
                        _playback.switchBy(-1);
                        _reveal();
                      },
              ),
              _round(
                _playback.paused
                    ? Icons.play_arrow_rounded
                    : Icons.pause_rounded,
                _playback.paused ? '播放' : '暂停',
                () {
                  _playback.togglePause();
                  _reveal();
                },
                strong: true,
              ),
              _round(
                Icons.skip_next_rounded,
                '下一台',
                _playback.channels.length < 2
                    ? null
                    : () {
                        _playback.switchBy(1);
                        _reveal();
                      },
              ),
              const Spacer(),
              if (_playback.count > 0)
                Text(
                  '${_playback.index + 1} / ${_playback.count}',
                  style: const TextStyle(color: Colors.white70, fontSize: 13),
                ),
              if (multi)
                _pill(
                  '线路 ${_playback.route + 1}/${_playback.routeCount}',
                  '切换线路',
                  () {
                    final next = (_playback.route + 1) % _playback.routeCount;
                    unawaited(_playback.playRoute(next));
                    _reveal();
                  },
                ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _round(
    IconData icon,
    String tooltip,
    VoidCallback? onPressed, {
    bool strong = false,
  }) => FocusRing(
    radius: 999,
    child: IconButton(
      tooltip: tooltip,
      onPressed: onPressed,
      style: IconButton.styleFrom(
        backgroundColor: strong
            ? Colors.white.withValues(alpha: .22)
            : Colors.black.withValues(alpha: .42),
        foregroundColor: Colors.white,
        disabledForegroundColor: Colors.white38,
        minimumSize: Size.square(strong ? 52 : 44),
      ),
      icon: Icon(icon),
    ),
  );

  Widget _pill(String text, String tooltip, VoidCallback onPressed) =>
      FocusRing(
        radius: 999,
        child: TextButton(
          onPressed: onPressed,
          style: TextButton.styleFrom(
            backgroundColor: Colors.black.withValues(alpha: .42),
            foregroundColor: Colors.white,
            shape: const StadiumBorder(),
            padding: const EdgeInsets.symmetric(horizontal: 16),
          ),
          child: Text(text),
        ),
      );
}

class _Shade extends StatelessWidget {
  const _Shade({required this.top});
  final bool top;

  @override
  Widget build(BuildContext context) => Positioned(
    left: 0,
    right: 0,
    top: top ? 0 : null,
    bottom: top ? null : 0,
    height: 120,
    child: IgnorePointer(
      child: DecoratedBox(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: top ? Alignment.topCenter : Alignment.bottomCenter,
            end: top ? Alignment.bottomCenter : Alignment.topCenter,
            colors: const [Color(0x99000000), Color(0x00000000)],
          ),
        ),
      ),
    ),
  );
}
