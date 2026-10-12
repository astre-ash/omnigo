package handler

const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>Metrics Service</title>
</head>
<body>
    <h1>Runtime Metrics</h1>
    <h2>Gauges</h2>
    <ul>
    {{range $name, $val := .Gauges}}
        <li><strong>{{$name}}</strong>: {{$val}}</li>
    {{end}}
    </ul>
    <h2>Counters</h2>
    <ul>
    {{range $name, $val := .Counters}}
        <li><strong>{{$name}}</strong>: {{$val}}</li>
    {{end}}
    </ul>
</body>
</html>`
