# powershell -ExecutionPolicy Bypass -File .\compile_proto.ps1
$ProtocPath="$env:USERPROFILE\local\protoc-3.13.0\bin"
$ProtoDefinition="dataputter\router.proto"

$args='--go_out=.',
  '--go_opt=paths=source_relative',
  '--go-grpc_out=.',
  '--go-grpc_opt=paths=source_relative',
  ${ProtoDefinition}

$cmd="${ProtocPath}\protoc.exe"

& $cmd $args