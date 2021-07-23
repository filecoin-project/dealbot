import "./jquery-global";
import "bootstrap-table";
import "bootstrap-cron-picker/dist/cron-picker";
import { FilecoinNumber } from "@glif/filecoin-number"
import prettyBytes from 'pretty-bytes'
import { data } from "jquery";

let auth = "";
let regionList = [];
window.setauth = (a) => {
    auth = a;
};

let modal = null
let shutdownDaemonID = ""
let shutdownDaemonIndex = 0
let notificationMessage = null
$().ready(() => {
    modal = new bootstrap.Modal(document.getElementById('confirmationModal'), {})
    notificationMessage = new bootstrap.Toast(document.getElementById('notificationMessage'), {})

    $('#newSchedule').cronPicker();
    $('#newretafterstoreSchedule').cronPicker();

    $("#addDone").hide();
    $("#addBotDone").hide();
    if ($('#newSR').is(':checked')) {
        $("#newstorage").hide();
    } else {
        $("#newretrieval").hide();
    }
    $("#newSR").on('change', () => {
        if ($('#newSR').is(':checked')) {
            $("#newretrieval").show();
            $("#newstorage").hide();
        } else {
            $("#newretrieval").hide();
            $("#newstorage").show();
        }
    })

    if (!$('#newRepeat').is(':checked')) {
        $("#setschedule").hide();
    }
    $("#newRepeat").on('change', () => {
        if ($('#newRepeat').is(':checked')) {
            $("#setschedule").show();
        } else {
            $("#setschedule").hide();
        }
    })

    if (!$('#newRepeatRetAfterStore').is(':checked')) {
        $("#retafterstoreschedule").hide();
    }
    $("#newRepeatRetAfterStore").on('change', () => {
        if ($('#newRepeatRetAfterStore').is(':checked')) {
            $("#retafterstoreschedule").show();
        } else {
            $("#retafterstoreschedule").hide();
        }
    })

    $("#addtask button").on('click', doSubmit);
    $("schedulesection form").on('submit', doSubmit);
    $("#addbot button").on('click', doCreateBot);
    $("#botlist").hide()

    $("#shutdownConfirm").on('click', confirmShutdown)
    fetch("./regions", { method: "GET", headers: getHeaders()}).then((response) => response.json()).then(gotRegions)
    syncData()
})


function syncData() {
    fetch("./tasks", { method: "GET", headers: getHeaders()}).then((response) => response.json()).then(gotTasks)
}

function operate(val, row) {
    return '<div class="remove btn btn-primary" title="remove">Cancel</div>';
}

let firstLoad = true

function gotTasks(data) {
    const processedData = data.map((item) => {
        let task = item.StorageTask || item.RetrievalTask
        const sched = {
            Schedule: task.Schedule,
            Limit: task.ScheduleLimit,
        }
        delete task.Schedule;
        delete task.ScheduleLimit;
        return Object.assign({ sched, task }, item)
    }).filter((item) => item.Status == 1 || item.sched.Schedule)
    if (firstLoad) {
        let stringify = (d) => JSON.stringify(d, null, 2);
        $("#taskTable").bootstrapTable({
            idField: 'UUID',
            columns: [
                {title:'ID', field:'UUID'},
                {title:'Task', field:'task', formatter: stringify},
                {title:'Schedule', field:'sched', formatter: stringify},
                {title:'Delete', field: 'operate', align: 'center', formatter: operate, events: { 'click .remove': cancel}}
            ],
            data: processedData,
        });
        firstLoad = false
    } else {
        $("#taskTable").bootstrapTable('load', processedData)
    }
}

let currentRegion = ""

function gotRegions(data) {
    regionList = data["regions"]
    $.each(regionList, (reg) => {
        $('<button/>')
            .addClass("list-group-item list-group-item-action")
            .text(regionList[reg])
            .on('click', function() {
                $("#regionList .list-group-item").removeClass("active")
                $(this).addClass("active")
                currentRegion = regionList[reg]
                fetch(`./regions/${regionList[reg]}`, { method: "GET", headers: getHeaders()}).then((response) => response.json()).then(gotDaemons)
            })
            .appendTo($("#regionlist"));
    });
}


function operateDaemon(val, row) {
    return `
        <div class="drain btn btn-primary" title="drain">Drain</div>
        <div class="reset btn btn-primary" title="reset">Reset</div>
        <div class="complete btn btn-primary" title="reset">Complete</div>
        <div class="shutdown btn btn-danger" title="shutdown">Shutdown</div>`;
}

let firstDaemonsLoad = true
function gotDaemons(data) {
    const daemons = data["daemons"]
    const processedDaemons = daemons.map((daemon) => {
        let daemonData = daemon.daemon
        daemonData.minfil = new FilecoinNumber(daemonData.minfil, 'attofil')
        daemonData.mincap = BigInt(daemonData.mincap)
        if (daemon.funds) {
            daemonData.balance = new FilecoinNumber(daemon.funds.balance, 'attofil')
            daemonData.datacap = BigInt(daemon.funds.datacap)
        }
        return daemonData
    })
    if (firstDaemonsLoad) {
        let filFormatter = (d) => d ? d.toFil() + " FIL" : "-"
        let capFormatter = (c) => c ? prettyBytes(Number(c)) : "-"
        let capStyler = (value, row) => {
            // assume no value + no data-cap = miner making unverified deals = no styling
            if (value == 0 && row.mincap == 0) {
                return {}
            }
            if (value < row.mincap) {
                return { classes: 'table-danger' }
            }
            if (value - row.mincap < ((1 << 30)*32)) {
                return { classes: 'table-warning' }
            }
            return { classes: 'table-success' }
        }
        let filStyler = (value, row) => {
            if (value < row.minfil) {
                return { classes: 'table-danger' }
            }
            if (value - row.minfil < 1) {
                return { classes: 'table-warning' }
            }
            return { classes: 'table-success' }
        }
        $("#botlist").show()

    $("#botdetail").bootstrapTable({
        idField: 'id',
        columns: [
            {title:'ID', field:'id'},
            {title:'Region', field:'region'},
            {title:'Wallet', field:'wallet.address'},
            {title: 'Balance', field: 'balance', formatter: filFormatter, cellStyle: filStyler},
            {title: 'Data Cap', field: 'datacap', formatter: capFormatter, cellStyle: capStyler},
            {title:'Actions', field: 'operate', align: 'center', formatter: operateDaemon, events: { 'click .drain': drain, 'click .reset': reset, 'click .complete': complete,'click .shutdown': shutdown}}
        ],
        data: processedDaemons,
		});
        firstDaemonsLoad = false
    } else {
        $("#botdetail").bootstrapTable('load', processedDaemons) 
    }
}

function errorToast(message) {
$('#notificationMessage').removeClass('bg-primary')
  $('#notificationMessage').addClass("bg-danger")
  $('#notificationMessage .toast-body').text(message)
  notificationMessage.show()
}

function successToast(message) {
    $('#notificationMessage').removeClass('bg-danger')
      $('#notificationMessage').addClass("bg-primary")
      $('#notificationMessage .toast-body').text(message)
      notificationMessage.show()
    }
    
function drain(e, value, row, index) {
    if (e.preventDefault) {
        e.preventDefault()
    }
    let url = `./drain/${row.id}`
    fetch(url, { method: "POST", headers: getHeaders()}).then((response) => {
      if (response.ok) {
        successToast(`Daemon ${row.id} will receive no new tasks`)
      } else {
        errorToast(`Error attempting to drain daemon ${row.id}`)
      }
    });
}

function reset(e, value, row, index) {
    if (e.preventDefault) {
        e.preventDefault()
    }
    let url = `./reset-worker/${row.id}`
    fetch(url, { method: "POST", headers: getHeaders()}).then((response) => {
        if (response.ok) {
          successToast(`All tasks on daemon ${row.id} will be reset to unstarted`)
        } else {
          errorToast(`Error attempting to reset daemon ${row.id}`)
        }
      });
}

function complete(e, value, row, index) {
    if (e.preventDefault) {
        e.preventDefault()
    }
    let url = `./complete/${row.id}`
    fetch(url, { method: "POST", headers: getHeaders()}).then((response) => {
        if (response.ok) {
          successToast(`Tasks for daemon ${row.id} will be published`)
        } else {
          errorToast(`Error attempting to publish tasks for daemon ${row.id}`)
        }
      });
}


function shutdown(e, value, row, index) {
    if (e.preventDefault) {
        e.preventDefault()
    }
    shutdownDaemonID = row.id
    shutdownDaemonIndex = index
    modal.show()
}

function confirmShutdown(e) {
    modal.hide()
   let url = `./regions/${currentRegion}/${shutdownDaemonID}`
    fetch(url, { method: "DELETE", headers: getHeaders()}).then((response) => {
        if (response.ok) {
            $("#botdetail").bootstrapTable('hideRow', { index: shutdownDaemonIndex })
            successToast(`Daemon ${row.id} has been shutdown`)
          } else {
            errorToast(`Error attempting to shutdown daemon ${row.id}`)
          }
    });
}


function cancel(e, value, row, index) {
    if (e.preventDefault) {
        e.preventDefault()
    }
    let url = `./tasks/${row.UUID}`
    fetch(url, { method: "DELETE", headers: getHeaders()}).then((response) => {
        if (response.ok) {
            $("#taskTable").bootstrapTable('hideRow', { index })
            successToast(`Task ${row.UUID} has been cancelled`)
          } else {
            errorToast(`Error attempting to cancel task ${row.id}`)
          }
    });
}

function getHeaders() {
    let headers = {
        'Content-Type': "application/json",
    }
    if (auth != "") {
       headers.Authorization =  `Basic ${btoa(auth)}`
    }
    return headers
}

function doSubmit(e) {
    if (e.preventDefault) {
        e.preventDefault()
    }

    // loop over miners
    let miners = $("#newMiner").val().trim().split('\n')

    let requests = []
    for (let i = 0; i < miners.length; i++) {
        let miner = miners[i];
        let url = "./tasks/storage";
        let data = {};
        if ($('#newSR').is(':checked')) {
            url = "./tasks/retrieval";
            data = {
                "Miner": miner,
                "PayloadCID": $('#newCid').val(),
                "CARExport": false,
                "MaxPriceAttoFIL": 20000000000,
            }
        } else {
            data = {
                "Miner": miner,
                "Size": Number($('#newSize').val()),
                "StartOffset": 6152, // 3 days?
                "FastRetrieval": $('#newFast').is(':checked'),
                "Verified": $('#newVerified').is(':checked'),
                "MaxPriceAttoFIL": 20000000000,
            }
            if ($('#newRepeatRetAfterStore').is(':checked')) {
                data.RetrievalSchedule = $('#newretafterstoreSchedule').val()
                data.RetrievalScheduleLimit = $('#newretafterstoreScheduleLimit').val()
            }
        }

        if ($('#newRepeat').is(':checked')) {
            data.Schedule = $('#newSchedule').val()
            data.ScheduleLimit = $('#newScheduleLimit').val()
        }
        if ($('#newScheduleTag').val() !='') {
            data.Tag = $('#newScheduleTag').val()
        }

        requests.push(fetch(url, {method: "POST", headers: getHeaders(), body: JSON.stringify(data)}))
    }

    Promise.all(requests).then((_) => {
        successToast("Tasks scheduled!")
        syncData()
    })

    return false
}

function doCreateBot(e) {
    let region = $("#newBotRegion").val()
    let url = `./regions/${region}`

    let data = {
        "id": $("#newBotId").val(),
        "tags": $("#newBotTags").val().trim().split('\n'),
        "workers": parseInt($("#newBotWorkers").val()),
        "mincap": $("#newBotMinCap").val(),
        "minfil": (new FilecoinNumber($("#newBotMinFil").val(), 'fil')).toAttoFil(),
        "wallet": {
            "address": $("#newBotWalletAddress").val(),
            "exported": $("#newBotWalletExported").val(),
        },
        "helmchartrepourl": $("#newBotHelmChartRepoUrl").val(),
        "helmchartversion": $("#newBotHelmChartVersion").val(),
        "lotusdockerrepo": $("#newBotLotusDockerRepo").val(),
        "lotusdockertag": $("#newBotLotusDockerTag").val()
    }
    fetch(url, {method: "POST", headers: getHeaders(), body: JSON.stringify(data)}).then(() => {
        successToast("Bot created!")
    })
}
