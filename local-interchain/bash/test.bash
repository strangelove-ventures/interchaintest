source ./source.bash

QUERY http://127.0.0.1:8080/ "localjuno-1" "bank total"

MAKE_REQUEST http://127.0.0.1:8080 "localjuno-1" "q" "bank total"