const { OPCUAServer, Variant, DataType, StatusCodes } = require("node-opcua");

const MACHINES = ["RS01", "RS02", "RS03"];

// 每台假機器的狀態。mode: 0待機 1烘焙中 2完成
const state = {};
MACHINES.forEach((id, i) => {
  state[id] = { temp: 60, target: 205, airflow: 0, mode: 0, left: 3 + i * 10 };
});

// 機器往前走一秒
function step(m) {
  const noise = Math.random() - 0.5;
  m.left--;
  if (m.mode === 0) {
    m.temp += (60 - m.temp) * 0.1 + noise * 0.2;
    m.airflow += (0 - m.airflow) * 0.3;
    if (m.left <= 0) { m.mode = 1; m.left = 90; }
  } else if (m.mode === 1) {
    m.temp += (m.target - m.temp) * 0.12 + noise;
    m.airflow += (60 - m.airflow) * 0.2 + noise * 2;
    if (m.left <= 0) { m.mode = 2; m.left = 5; }
  } else if (m.mode === 2) {
    m.temp += (60 - m.temp) * 0.03;
    m.airflow += (0 - m.airflow) * 0.5;
    if (m.left <= 0) { m.mode = 0; m.left = 15; }
  } else {
    // 故障:加熱器卡住,溫度一直升
    m.temp += 4;
    m.airflow += 1;
    if (m.left <= 0) { m.mode = 0; m.left = 15; }
  }
  m.airflow = Math.max(0, m.airflow);
}
setInterval(() => MACHINES.forEach((id) => step(state[id])), 1000);

const readline = require("readline");
readline.emitKeypressEvents(process.stdin);
if (process.stdin.isTTY) process.stdin.setRawMode(true);
process.stdin.on("keypress", (str, key) => {
  if (key && key.ctrl && key.name === "c") {
    process.emit("SIGINT");
    return;
  }
  if (str === "f") {
    const m = state.RS01;
    m.mode = 3;
    m.left = 15;
    m.temp = Math.max(m.temp, 230);
    console.log("[故障] RS01 加熱器卡住了");
  }
});

async function main() {
  const server = new OPCUAServer({
    port: 4840,
    resourcePath: "/UA/RoastMES",
    buildInfo: { productName: "RoastMES-FakePLC", buildNumber: "1" },
  });
  await server.initialize();

  const addressSpace = server.engine.addressSpace;
  const ns = addressSpace.registerNamespace("urn:roast-mes");

  // 在伺服器上放一個資料點。write 有給的話,這個點可以被客戶端寫入。
  function addVar(device, id, name, read, write) {
    const options = {
      componentOf: device,
      nodeId: `s=${id}.${name}`,
      browseName: name,
      dataType: "Double",
      minimumSamplingInterval: 500,
      value: {
        get: () => new Variant({ dataType: DataType.Double, value: read() }),
      },
    };
    if (write) {
      options.accessLevel = "CurrentRead | CurrentWrite";
      options.userAccessLevel = "CurrentRead | CurrentWrite";
      options.value.set = (variant) => {
        write(variant.value);
        return StatusCodes.Good;
      };
    }
    const v = ns.addVariable(options);
    console.log("  ", v.nodeId.toString());
  }

  console.log("建立的資料點(NodeId):");
  for (const id of MACHINES) {
    const m = state[id];
    const device = ns.addObject({
      organizedBy: addressSpace.rootFolder.objects,
      browseName: id,
    });
    addVar(device, id, "Temperature", () => Math.round(m.temp * 10) / 10);
    addVar(device, id, "Airflow", () => Math.round(m.airflow * 10) / 10);
    addVar(device, id, "CycleState", () => m.mode);
    addVar(device, id, "RemainingSec", () => (m.mode === 1 ? m.left : 0));
    addVar(device, id, "SetTemperature", () => m.target, (v) => {
      m.target = v;
      console.log(`[寫入] ${id} 目標溫度改為 ${v}`);
    });
  }

  await server.start();
  console.log("\n假 PLC 已啟動:", server.getEndpointUrl());
  console.log("按 Ctrl+C 結束\n");

  // 每 5 秒印一次 RS01 的狀況,方便確認它有在動
  setInterval(() => {
    const m = state.RS01;
    const mode = ["待機", "烘焙中", "完成", "故障"][m.mode];
    console.log(`RS01  ${mode}  豆溫 ${m.temp.toFixed(1)}  目標 ${m.target}`);
  }, 5000);

  process.on("SIGINT", async () => {
    await server.shutdown();
    process.exit(0);
  });
}

main().catch((err) => {
  console.error("啟動失敗:", err);
  process.exit(1);
});