Простой инструмент для симуляции дата рейса реквестами через TCP запросы

запросы доходят практически одновременно за счет отправки последних 2х байт сразу из всех горутин

пример запроса:


```
curl --location 'localhost:8080/multiTCP?port=443&count=100&next_url=https%3A%2F%2Fhttpbin.org%2Fanything' \
--header 'Host: httpbin.org' \
--header 'Content-Type: text/plain' \
--data 'sdffsfds'
```

count - количество запросов
port - порт получателя
next_url - url на который будут слаться запросы

Обязательно указывать Host в хедерсах.

ответ в формате json: {"responses": [{"response": "string", "error": "string"}, ...]}
