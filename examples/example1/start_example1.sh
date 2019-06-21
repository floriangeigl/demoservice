pkill s1
pkill s2
pkill s3
pkill s4
pkill s5
pkill s6

sleep 5

cp ../../service s1
cp ../../service s2
cp ../../service s3
cp ../../service s4
cp ../../service s5
cp ../../service s6

./s1 8001&
./s2 8002&
./s3 8003&
./s4 8004&
./s5 8005&
./s6 8006&

sleep 5

curl -i -X POST \
   -H "Content-Type:application/json" \
   -d \
'{
  "Callees" : [
    { "Adr" : "http://localhost:8002", "Count" : 2 },
    { "Adr" : "http://localhost:8003", "Count" : 6 },
    { "Adr" : "http://www.example.com", "Count" : 1 }
  ]
}' \
 'http://localhost:8001/config'

curl -i -X POST \
   -H "Content-Type:application/json" \
   -d \
'{
  "Callees" : [
    { "Adr" : "http://localhost:8004", "Count" : 2 },
    { "Adr" : "http://localhost:8005", "Count" : 6 },
    { "Adr" : "http://www.example.com", "Count" : 1 }
  ]
}' \
 'http://localhost:8003/config'

curl -i -X POST \
   -H "Content-Type:application/json" \
   -d \
'{
  "Callees" : [
    { "Adr" : "http://localhost:8006", "Count" : 2 },
    { "Adr" : "http://www.example.com", "Count" : 1 }
  ]
}' \
 'http://localhost:8005/config'
