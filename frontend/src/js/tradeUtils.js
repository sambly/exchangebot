import $ from 'jquery'
import { timeToLocal } from './help.js';
import { lw_charts_orders, lw_charts_volume, widget_charts } from './charts.js';

function update_top_data(pair) {
    $.ajax({
        url: '/trade/api/updateTop',
        type: 'POST',
        method: 'POST',
        data: pair,
        cache: false,
        contentType: ' text/html; charset=utf-8',
        processData: false,
        success: function (response) {
            // Загаловки 24ch  Volume
            let ch24Top = document.querySelector('#ch24-top');
            ch24Top.innerHTML = (response.Ch24).toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' }) + ' %';
            let VolumeTop = document.querySelector('#volume-top');
            VolumeTop.innerHTML = (response.Volume).toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' });;
        },
        error: function (response) {
        },
    });
}

export async function chart_volume_update() {

    var start = performance.now();

    let pair = document.querySelector('#pairs');
    let frames = document.querySelectorAll('.btnFrame');
    let frame;
    let checboxesActive = [];

    let checkboxes = document.querySelectorAll('[name="change-delta-check"]');
    checkboxes.forEach((checkbox, index) => {
        if (checkbox.checked) {
            checboxesActive.push({name:checkbox.value});
        }
    })

    for (let f of frames) {
        if (f.classList.contains('active')) {
            frame = f;
        }
    }

    let container_chart = document.getElementById('chart-volume');
    container_chart.innerHTML = `
    <div class="container d-flex justify-content-center align-items-center" style="height: 468px;">
      <div class="spinner-border" role="status">
        <span class="visually-hidden">Loading...</span>
      </div>
    </div>
  `;
                                                                
    let chartWidth = container_chart.clientWidth;

    const chartOptions = {
        layout: {
            textColor: 'black',
            background: { type: 'solid', color: 'white' },
        },
        autosize : true,
        height: 468,
        width: chartWidth,
        timeScale: {
            timeVisible: true,
            secondsVisible: false
        },
    };

    function update_volume_data(pair, frame, checboxesActive) {
        return new Promise((resolve, reject) => {
            let request = { Pair: pair, Frame: frame };
            let chartData = {};
            $.ajax({
                url: '/trade/api/getChangeDelta',
                type: 'POST',
                method: 'POST',
                data: JSON.stringify(request),
                cache: false,
                contentType: 'application/json; charset=utf-8',
                processData: false,
                success: function (data) {

                    checboxesActive.forEach(item => {
                        chartData[item.name] = { value: [] };
                    });

                    for (let d of data) {
                        checboxesActive.forEach(item => {
                            chartData[item.name].value.push({ time: timeToLocal(new Date(d['Time']) / 1000), value: d[item.name] })
                        });
                    }
                    resolve(chartData);
               
                },
                error: function (response) {
                    reject(response);
                },
            });
        });
    }

    try {        
        let chartData = await update_volume_data(pair.value, frame.innerText, checboxesActive);
        lw_charts_volume(container_chart, chartOptions, pair.value, chartData);
         // Расчет времени выполнения 
        var end = performance.now();
        var time = end - start;
        console.log('Время выполнения chart_volume_update = ' + time); 
    } catch (error) {
        console.error('Ошибка в chart_volume_update:', error);
    }

}

export function change_pair(pair) {

    var start = performance.now();

    let currentPair = localStorage.getItem('currentPair');
    let widgetPair = localStorage.getItem('widgetPair');
    let update_widget = false;
    if (widgetPair !== pair) {
        update_widget = true;
    }
    if (pair === '') {
        pair = currentPair;
    }
    if (pair !== currentPair) {
        localStorage.setItem('currentPair', pair);
    }
    document.querySelector('#pairs').value = pair;

    // Перемещение курсора в списке цен
    if ($('#list-ch-price').css('display') == "block") {
        var rowsPrice = document.querySelector("#tbody-price").rows;
        for (let i = 0; i < rowsPrice.length; i++) {
            rowsPrice[i].classList.remove('table-tr-active');
            if (rowsPrice[i].querySelector("td[name=pair]").innerHTML === pair.split("USDT")[0]) {
                rowsPrice[i].classList.add('table-tr-active');
                rowsPrice[i].scrollIntoView({
                    behavior: 'auto',
                    block: 'center'
                });
            }
        }

        if (update_widget) {
            widget_charts(document.getElementById('chart-price'), pair)
        }

    }

    // Перемещение курсора в списке объемов
    if ($('#list-ch-volume').css('display') == "block") {
        var rowsVolume = document.querySelector("#tbody-delta").rows;
        for (let i = 0; i < rowsVolume.length; i++) {
            rowsVolume[i].classList.remove('table-tr-active');
            if (rowsVolume[i].querySelector("td[name=pair]").innerHTML === pair.split("USDT")[0]) {
                rowsVolume[i].classList.add('table-tr-active');
                rowsVolume[i].scrollIntoView({
                    behavior: 'auto',
                    block: 'center'
                });
            }
        }

        chart_volume_update().then();
    }

    update_top_data(pair);

    // Расчет времени выполнения 
    var end = performance.now();
    var time = end - start;
    console.log('Время выполнения change_pair = ' + time);

}

export function show_chart_orders() {

    $("#chart-price").hide();
    $("#panel-chart-volume").hide();

    $("#panel-chart-orders").show();
}

export async function chart_frome_orders_update(orders) {

    return new Promise((resolve, reject) => {

        let pair = document.querySelector('#pairs');
        let container_chart = document.getElementById('chart-orders');
        let chartWidth = container_chart.clientWidth;
        let chartHeight = 468;
        const chartOptions = {
            height: chartHeight,
            width: chartWidth,
            autosize: true,
            layout: {
                backgroundColor: '#ffffff',
                textColor: 'rgba(33, 56, 77, 1)',
            },
            grid: {
                vertLines: {
                    color: 'rgba(197, 203, 206, 0.7)',
                },
                horzLines: {
                    color: 'rgba(197, 203, 206, 0.7)',
                },
            },
            timeScale: {
                timeVisible: true,
                secondsVisible: false
            },
        };

        function update_candles(pair, frame) {

            return new Promise((resolve, reject) => {
                let request = { Pair: pair, Frame: frame };
                let candles = [];
                $.ajax({
                    url: '/trade/api/getChangeDelta',
                    type: 'POST',
                    method: 'POST',
                    data: JSON.stringify(request),
                    cache: false,
                    contentType: 'application/json; charset=utf-8',
                    processData: false,
                    success: function (data) {
                        for (let item of data) {
                            candles.push({ time: timeToLocal(new Date(item['Time']) / 1000), open: item['Open'], high: item['High'], low: item['Low'], close: item['Close'] })
                        }
                        resolve(candles);
                    },
                    error: function (response) {
                        reject(response);
                    },
                });
            });
        }

        lw_charts_orders(container_chart, chartOptions, pair, orders, update_candles).then(() => {
            resolve();
        }).catch(error => {
            reject(error);
        });
    });
}