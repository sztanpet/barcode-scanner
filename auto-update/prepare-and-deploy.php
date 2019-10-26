#!/bin/env php
<?php
error_reporting(E_ALL);
ini_set('display_errors', '1');

define('BASE_PATH', realpath( __DIR__ . '/..' ) . '/' );
define('BUCKET_PATH', realpath( __DIR__ . '/bucket' ) . '/' );
include('config.php');
if ( !defined("S3_BUCKET") )
    throw new \Exception("S3_BUCKET constant missing from config.php! Example: <?php define('S3_BUCKET', 's3://foobar/');");

$binaries = [
    "barcode-scanner",
    "error-checker",
    "updater",
];


if ( $argc > 1 ) {
  logMsg("More than 1 argument passed, assuming build-only mode!");
}

foreach( $binaries as $bin ) {
    buildBinary( $bin );
    copyBinaryToBucket( $bin );
    assembleVersion( $bin );
    logMsg(" ");
}

if ( $argc > 1 ) {
  logMsg("Skipping upload to S3!");
  logMsg("Done!");
  return;
}

uploadToS3();
logMsg("Done!");

// -----------
function buildBinary( $binary ) {
    logMsg("Building binary: {$binary}...");
    $path = BASE_PATH . "cmd/{$binary}";
    check( chdir( $path ), "buildBinary - {$path}" );
    run("env GOOS=linux GOARCH=arm GOARM=5 go build");
}

function copyBinaryToBucket( $binary ) {
    $binPath = BASE_PATH . "cmd/{$binary}/{$binary}";
    $dest = BUCKET_PATH . "{$binary}/linux-arm/{$binary}";
    check( file_exists( $binPath ), "copyBinaryToBucket - {$binary}" );

    if ( !file_exists( dirname( $dest ) ) )
        check( mkdir( dirname( $dest ), 0755, true ), "mkdir - {$binary}" );

    logMsg("Copying binary to bucket ({$binPath} => {$dest})");
    check( copy( $binPath, $dest ), "copy - {$binPath} => {$dest}" );
}

function assembleVersion( $binary ) {
    $binPath = BASE_PATH . "cmd/{$binary}/{$binary}";
    check( file_exists( $binPath ), "assembleVersion - {$binary}" );

    $hash = hash('sha256', file_get_contents( $binPath ) );
    $json = [
        "hash" => $hash,
        "binaryPath" => $binary,
    ];

    $verPath = BUCKET_PATH . "{$binary}/linux-arm/version.json";
    logMsg("Preparing version.json ({$verPath})");
    check( file_put_contents( $verPath, json_encode( $json ) ), "json - $verPath" );
}

function uploadToS3() {
    chdir( __DIR__ ); // just because it makes the aws messages cleaner
    logMsg("Uploading bucket to S3...");
    run("aws s3 sync " . BUCKET_PATH . " " . S3_BUCKET . " --delete");
}

function logMsg( $msg ) {
    if ( !$msg )
        return;

    echo $msg, "\n";
}

function run( $cmd ) {
    passthru ( $cmd, $exitCode );
    if ( $exitCode != 0 ) {
      logMsg("Exit code was non-zero ({$exitCode})!");
      throw new \Exception("cmd failed: {$cmd}");
    }
}

function check( $result, $msg ) {
    if ( !$result )
        throw new \Exception( $msg );
}
