"""
Step 03: Python gRPC クライアント

前提: Go サーバーが localhost:50051 で起動していること
  cd ../server-go && go run .

使用方法:
  pip install -r requirements.txt
  python client.py
"""

import os
import sys

import grpc

# grpc_tools.protoc で生成されたファイルをインポート
# 事前に `bash ../../scripts/gen-python.sh` を実行してください
#
# 生成コード（greeter_pb2_grpc.py）は `from step03 import greeter_pb2` という
# 絶対インポートを行うため、gen/ ディレクトリを sys.path に追加しておく必要がある。
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "gen"))

try:
    from step03 import greeter_pb2, greeter_pb2_grpc
except ImportError:
    print("エラー: 生成済みコードが見つかりません。")
    print("以下を実行して Python スタブを生成してください:")
    print("  bash ../../scripts/gen-python.sh")
    sys.exit(1)


def run_sync():
    """同期クライアント（基本パターン）"""
    print("=== 同期クライアント ===")
    with grpc.insecure_channel("localhost:50051") as channel:
        stub = greeter_pb2_grpc.GreeterServiceStub(channel)

        names = ["太郎", "Alice", "Bob"]
        for name in names:
            try:
                response = stub.SayHello(greeter_pb2.SayHelloRequest(name=name))
                print(f"サーバーの返答: {response.message}")
                print(f"サーバーホスト: {response.server_host}")
            except grpc.RpcError as e:
                print(f"gRPC エラー: code={e.code()}, details={e.details()}")


async def run_async():
    """非同期クライアント（async/await パターン）"""
    import grpc.aio

    print("\n=== 非同期クライアント ===")
    async with grpc.aio.insecure_channel("localhost:50051") as channel:
        stub = greeter_pb2_grpc.GreeterServiceStub(channel)
        response = await stub.SayHello(greeter_pb2.SayHelloRequest(name="非同期ユーザー"))
        print(f"非同期レスポンス: {response.message}")
        print(f"サーバーホスト: {response.server_host}")


if __name__ == "__main__":
    import asyncio

    run_sync()
    asyncio.run(run_async())

    print("\n=== 完了 ===")
