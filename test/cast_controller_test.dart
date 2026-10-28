import 'package:duanju_app/cast/cast_controller.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  final controller = CastController.instance;
  late List<MethodCall> calls;

  void mock(Future<Object?> Function(MethodCall call) handler) {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(CastController.channel, (call) async {
          calls.add(call);
          return handler(call);
        });
  }

  setUp(() {
    calls = [];
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    controller.reset();
  });

  tearDown(() {
    controller.reset();
    debugDefaultTargetPlatformOverride = null;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(CastController.channel, null);
  });

  test('搜索设备：把原生返回的列表解析成设备对象', () async {
    mock(
      (call) async => {
        'devices': [
          {
            'name': '客厅电视',
            'udn': 'uuid:renderer-1',
            'controlUrl': 'http://192.168.1.5:4004/upnp/control/AVTransport1',
          },
        ],
        'error': '',
      },
    );

    await controller.scan();

    expect(calls.single.method, 'discover');
    expect(controller.scanning, isFalse);
    expect(controller.devices.single.name, '客厅电视');
    expect(controller.devices.single.controlUrl, contains('/AVTransport1'));
    expect(controller.error, isNull);
  });

  test('搜索不到设备时保留原生给的提示', () async {
    mock(
      (call) async => {
        'devices': <Object>[],
        'error': '没有搜索到可投屏的设备，请确认电视已开机并与手机处于同一 Wi-Fi',
      },
    );

    await controller.scan();

    expect(controller.devices, isEmpty);
    expect(controller.error, contains('同一 Wi-Fi'));
  });

  test('投屏：下发地址、暂停、停止都带正确的参数', () async {
    mock((call) async => null);
    const device = CastDevice(
      name: '客厅电视',
      udn: 'uuid:renderer-1',
      controlUrl: 'http://192.168.1.5:4004/upnp/control/AVTransport1',
    );

    await controller.start(
      target: device,
      url: 'http://192.168.1.9:5321/token/index.m3u8',
      title: '测试剧 第1集',
      position: const Duration(seconds: 42),
    );
    expect(controller.casting, isTrue);
    expect(controller.device?.name, '客厅电视');
    final play = calls.first;
    expect(play.method, 'play');
    expect(play.arguments['url'], 'http://192.168.1.9:5321/token/index.m3u8');
    expect(play.arguments['title'], '测试剧 第1集');
    expect(play.arguments['positionMs'], 42000);

    await controller.pause();
    expect(controller.playing, isFalse);
    expect(calls.last.arguments['command'], 'pause');

    await controller.resume();
    expect(controller.playing, isTrue);
    expect(calls.last.arguments['command'], 'resume');

    await controller.stop();
    expect(controller.casting, isFalse);
    expect(controller.device, isNull);
    expect(calls.last.arguments['command'], 'stop');
  });

  test('非 Android 平台不发起投屏调用', () async {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    mock((call) async => null);

    expect(controller.supported, isFalse);
    await controller.scan();

    expect(calls, isEmpty);
    expect(controller.devices, isEmpty);
  });
}
