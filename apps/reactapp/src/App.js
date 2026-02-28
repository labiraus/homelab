import { useState } from "react";
import "./App.css";
import logo from "./logo.svg";

function App() {
	const [output, setOutput] = useState();

	const goHello = async () => {
		console.log("calling hello");
		const res = await fetch("/go/hello");
		console.log(res);
		if (res.status !== 200) {
			alert(res.statusText);
			return;
		}
		const response = await res.json();
		setOutput(response.data);
	};

	const requestOptions = {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ title: "Fetch POST Request Example" }),
	};

	const pythonHello = async () => {
		console.log("calling python");
		const res = await fetch("/python/hello", requestOptions);
		console.log(res);
		if (res.status !== 200) {
			alert(res.statusText);
			return;
		}
		const response = await res.json();
		setOutput(response.data);
	};

	const javaHello = async () => {
		console.log("calling java");
		const res = await fetch("/java/hello", requestOptions);
		console.log(res);
		if (res.status !== 200) {
			alert(res.statusText);
			return;
		}
		const response = await res.json();
		setOutput(response.data);
	};

	const nodeHello = async () => {
		console.log("calling node");
		const res = await fetch("/node/hello", requestOptions);
		console.log(res);
		if (res.status !== 200) {
			alert(res.statusText);
			return;
		}
		const response = await res.json();
		setOutput(response.data);
	};

	const dotnetHello = async () => {
		console.log("calling dotnet");
		const res = await fetch("/dotnet/hello", requestOptions);
		console.log(res);
		if (res.status !== 200) {
			alert(res.statusText);
			return;
		}
		const response = await res.json();
		setOutput(response.data);
	};

	return (
		<div className="App">
			<header className="App-header">
				<img src={logo} className="App-logo" alt="logo" />
				<button onClick={goHello}>go click me</button>
				<button onClick={pythonHello}>python click me</button>
				<button onClick={javaHello}>java click me</button>
				<button onClick={nodeHello}>node click me</button>
				<button onClick={dotnetHello}>dotnet click me</button>
				<p id="output">{output}</p>
			</header>
		</div>
	);
}

export default App;
