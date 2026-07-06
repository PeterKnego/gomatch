#!/usr/bin/env bash
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
JAR="${SBE_JAR:-$DIR/sbe-all-1.38.1.jar}"
if [ ! -f "$JAR" ]; then
  curl -fL -o "$JAR" https://repo1.maven.org/maven2/uk/co/real-logic/sbe-all/1.38.1/sbe-all-1.38.1.jar
fi
java -Dsbe.output.dir="$DIR" -Dsbe.target.language=golang \
     -Dsbe.target.namespace=codecs -Dfile.encoding=UTF-8 \
     -jar "$JAR" "$DIR/gomatch-schema.xml"
