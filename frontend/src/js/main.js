


import '../scss/styles.scss';
import '../css/style.css';

import * as bootstrap from 'bootstrap'
import $ from 'jquery'
window.jQuery = window.$ = $


import { lw_charts_orders, lw_charts_volume, widget_charts } from './charts.js';
import { timeToLocal } from './help.js';

import { createApp, h } from 'vue'
import OrdersTable from '../components/OrdersTable.vue'
import OrdersTableHistory from '../components/OrdersTableHistory.vue'

import { createPinia } from 'pinia';
import { useOrdersStore } from '../stores/orders.js';


import emitter from './eventBus';

// Инициализация Pinia
const pinia = createPinia();

// Приложение для активных ордеров
const activeApp = createApp({
  render: () => h(OrdersTable)
});
activeApp.use(pinia);
activeApp.mount('#panel-trade-active');

// Приложение для истории ордеров
const historyApp = createApp({
  render: () => h(OrdersTableHistory)
});
historyApp.use(pinia);
historyApp.mount('#panel-trade-history');

// Глобальные функции обновления ордеров
window.forming_orders_active = function(orders) {
  useOrdersStore().setActive(orders);
};

window.forming_orders_history = function(orders) {
  useOrdersStore().setHistory(orders);
};

$(function () {
    

    const grafanaUrl = import.meta.env.VITE_GRAFANA_URL;
    document.getElementById('grafana-link').href = grafanaUrl;

    document.getElementById('jaeger-link').href = "jaeger";

    forming_page();

    //#############################################################################  webSocket #############################################################################


    var Url = (window.location.protocol === "https:" ? "wss://" : "ws://") + window.location.host + "/trade/ws";
    const socket = new WebSocket(Url);

    socket.onmessage = function (e) {
      const data = JSON.parse(e.data);
    
      if (data.orderUpdate) {
        emitter.emit('order:update', data.orderUpdate);
      }
      if (data.orderAdd) {
        emitter.emit('order:add', data.orderAdd);
      }
      if (data.orderDelete) {
        emitter.emit('order:remove', data.orderDelete);
      } 
      if (data.pnl) {
        emitter.emit('pnl:update', data.pnl);
      }
    };

    socket.onerror = (error) => {
        console.log(`WebSocket Error: ${error.message}`);
    };

    socket.onclose = (event) => {
        console.log("Connection closed");
    };


    //#############################################################################  webSocket #############################################################################

    // Сплывающее уведомление
    var myToast = new bootstrap.Toast(document.querySelector('.toast'), {
        animation: true, // Включить анимацию при отображении/скрытии всплывающего уведомления
        autohide: true, // Автоматически скрывать всплывающее уведомление после указанной задержки
        delay: 3000 // Задержка перед автоматическим скрытием всплывающего уведомления (в миллисекундах)
    });
    $("#toastMessage").text("");


    // Аткинвые кнопки меню (Цена,Объем)
    $('.btnMenu').click(function () {
        $('.btnMenu').removeClass('active'); // Удаляем класс 'active' у всех кнопок
        $(this).addClass('active'); // Добавляем класс 'active' текущей кнопке
    });

    // Меню цены
    $('#btn-price').click(function () {
        show_price_panel();
        change_pair(document.querySelector('#pairs').value)
    });

    // Меню объема
    $('#btn-volume').click(function () {
        show_volume_panel();
        change_pair(document.querySelector('#pairs').value);
    });

    // Аткинвые кнопки ордеров
    $('.btnTradeHistory').click(function () {
        $('.btnTradeHistory').removeClass('active');
        $(this).addClass('active');
    });

    $('#btn-trade-active').click(function () {
        show_panel_trade_active();
    });
    $('#btn-trade-history').click(function () {
        show_panel_trade_history();
    });
    // Аткинвые кнопки выбора пар
    $('.btnPairs').click(function () {
        $('.btnPairs').removeClass('active');
        $(this).addClass('active');
        forming_tickers_list_All();
    });

    // Изменение фрейма для tickers volume
    $('.btnFrame').click(function (e) {

        $('.btnFrame').removeClass('active');
        $(this).addClass('active');

        forming_tickers_list_volume();
        change_pair(document.querySelector('#pairs').value);
    });

    $('#btn-update-data').click(function (e) {
        e.preventDefault();
        e.target.disabled = true;
        $.ajax({
            url: 'updatefull',
            type: 'POST',
            method: 'POST',
            cache: false,
            contentType: 'application/json; charset=utf-8',
            processData: false,
            success: function (response) {
                update_main_data(
                    response.MarketsStat,
                );
                $("#toastMessage").text("Данные загружены");
                myToast.show();
                e.target.disabled = false;
            },
            error: function (response) {
            },

        });
    });

    // Аткинвые кнопки меню
    $('.btnFrameStrategy').click(function () {
        $('.btnFrameStrategy').removeClass('active');
        $(this).addClass('active');
    });

    // Открыть long/short позицию
    $('.btn-trade-deal').click(function (e) {
        e.preventDefault();

        let sideType

        if (e.target.id == "panel-trdae-open-deal") {
            sideType = "BUY";
        }
        if (e.target.id == "panel-trdae-close-deal") {
            sideType = "SELL";
        }

        // default
        let frame = '15m';
        let btnFrameStrategy = document.querySelectorAll('.btnFrameStrategy');
        btnFrameStrategy.forEach((btn, index) => {
            if (btn.classList.contains("active")) {
                frame = btn.innerText;
            }
        });


        let form = {
            pair: document.querySelector('#pairs').value,
            sideType: sideType,
            frame: frame,
            strategy: document.querySelector('#panel-trade-strategy').value,
            comment: document.querySelector('#panel-trade-comment').value
        };


        $.ajax({
            url: 'openDeal',
            type: 'POST',
            method: 'POST',
            cache: false,
            contentType: 'application/json; charset=utf-8',
            processData: false,
            data: JSON.stringify(form),
            success: function (orders) {
                forming_orders_active(orders);
            },
            error: function (response) {
                console.log(response);
            },

        });
    });
      
    // Применить фильтры
    $('#btn-apply-filter').click(function () {
        // Объем 
        let minVolume = document.querySelector('#minVolume').value.trim() || null;
        let maxVolume = document.querySelector('#maxVolume').value.trim() || null;;
        localStorage.setItem('minVolume', minVolume);
        localStorage.setItem('maxVolume', maxVolume);
        // ch1d
        let minCh1d = document.querySelector('#minCh1d').value.trim() || null;
        let maxCh1d = document.querySelector('#maxCh1d').value.trim() || null;;
        localStorage.setItem('minCh1d', minCh1d);
        localStorage.setItem('maxCh1d', maxCh1d);

    });

    // Сбросить фильтры
    $('#btn-reset-filter').click(function () {
        // Объем 
        document.querySelector('#minVolume').value = null;
        document.querySelector('#maxVolume').value = null;
        localStorage.setItem('minVolume', null);
        localStorage.setItem('maxVolume', null);
        // ch1d
        document.querySelector('#minCh1d').value = null;
        document.querySelector('#maxCh1d').value = null;
        localStorage.setItem('minCh1d', null);
        localStorage.setItem('maxCh1d', null);
    });


});


function forming_page() {

    size_conversion();

    $.ajax({
        url: 'formingPage',
        async: false,
        type: 'POST',
        method: 'POST',
        cache: false,
        contentType: 'application/json; charset=utf-8',
        processData: false,
        success: function (response) {

            let pairs = response.Pairs;
            let marketsStat = response.MarketsStat;
            let ordersActive = response.OrdersActive;
            let ordersHistory = response.OrdersHistory;
            let strategyDescription = response.OptionStrategy;

            // Select pairs
            let selectPairs = document.querySelector('#pairs');
            let selectPairsList = document.querySelector('#pairslistOptions');
            for (let index in pairs) {
                let option = new Option(pairs[index], pairs[index]);
                selectPairsList.prepend(option)
            }
            selectPairs.addEventListener('change', (e) => {
                change_pair(e.target.value);
            });

            // option Strategy
            let selectStrategy = document.querySelector('#panel-trade-strategy');

            for (let optionName in strategyDescription) {
                let option = new Option(optionName, optionName);
                option.setAttribute("title", strategyDescription[optionName].description);
                selectStrategy.prepend(option);
            }

            // выбор текущей пары
            let currentPair = localStorage.getItem('currentPair') || 'BTCUSDT';
            localStorage.setItem('currentPair', currentPair);

            // Фильтры
            // Объем 
            document.querySelector('#minVolume').value = localStorage.getItem('minVolume');
            document.querySelector('#maxVolume').value = localStorage.getItem('maxVolume');
            // Ch1d
            document.querySelector('#minCh1d').value = localStorage.getItem('minCh1d');
            document.querySelector('#maxCh1d').value = localStorage.getItem('maxCh1d');

            // Выбор определенного типа графика
            let checkboxes = document.querySelectorAll('[name="change-delta-check"]');
            checkboxes.forEach((checkbox, index) => {
                checkbox.addEventListener('change', (e) => {
                    // // Сбросить все галочки 
                    // checkboxes.forEach((checkboxClear, index) => {
                    //     checkboxClear.checked = false;
                    // });
                    // checkbox.checked = true;

                    chart_volume_update().then();
                })
            })

            // Обновление данных 
            update_main_data(marketsStat);

            // Формирование panel-trade
            forming_orders_active(ordersActive);
            forming_orders_history(ordersHistory);

            show_panel_trade_active();


        },
        error: function (response) {
        },

    });
}

function size_conversion() {
    var windowWidth = $('body').innerWidth();

    if (windowWidth < 576) {                                                                // class none
        $('.btn').removeClass('btn-sm');
        $("#trades").removeAttr("class");
        $('#trades').addClass('d-flex flex-column gap-4 justify-content-between');
        $("#header_trades").removeAttr("class");
        $('#header_trades').addClass('d-flex flex-column gap-4 justify-content-between');

    } else if (windowWidth >= 576 && windowWidth < 768) {                                  // class sm
        $('.btn').removeClass('btn-sm');
        $("#trades").removeAttr("class");
        $('#trades').addClass('d-flex flex-column gap-4 justify-content-between');
        $("#header_trades").removeAttr("class");
        $('#header_trades').addClass('d-flex flex-column gap-4 justify-content-between');
    } else if (windowWidth >= 768 && windowWidth < 992) {                                  // class md
        $('.btn').addClass('btn-sm');
        $("#trades").removeAttr("class");
        $('#trades').addClass('d-flex');
        $("#header_trades").removeAttr("class");
        $('#header_trades').addClass('d-flex gap-2 align-items-start');
    } else if (windowWidth >= 992 && windowWidth < 1200) {                                 // class lg
        $('.btn').addClass('btn-sm');
        $("#trades").removeAttr("class");
        $('#trades').addClass('d-flex gap-2 align-items-start justify-content-between');
        $("#header_trades").removeAttr("class");
        $('#header_trades').addClass('d-flex gap-2 align-items-start');
    } else if (windowWidth >= 1200 && windowWidth <= 1432) {                                 // class xl
        $('.btn').addClass('btn-sm');
        $("#trades").removeAttr("class");
        $('#trades').addClass('d-flex gap-2 align-items-start justify-content-between');
        $("#header_trades").removeAttr("class");
        $('#header_trades').addClass('d-flex gap-2 align-items-start');
    } else if (windowWidth > 1432) {                                                        // class xxl
        $('.btn').removeClass('btn-sm');
        $("#trades").removeAttr("class");
        $('#trades').addClass('d-flex align-items-start justify-content-between');
        $("#header_trades").removeAttr("class");
        $('#header_trades').addClass('d-flex gap-2 align-items-start');
    }
}

function update_main_data(marketsStat) {

    let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];
    localStorage.setItem('favoritePairs', JSON.stringify(favoritePairs));

    forming_tickers_list_All();

    // Загаловки 24ch  Volume
    let selectPairs = document.querySelector('#pairs');
    let ch24Top = document.querySelector('#ch24-top');
    ch24Top.innerHTML = (marketsStat[selectPairs.value].Ch24).toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' }) + '%';
    let VolumeTop = document.querySelector('#volume-top');
    VolumeTop.innerHTML = (marketsStat[selectPairs.value].Volume).toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' });

}

function forming_tickers_list_All() {

    $("#list-ch-price").show();
    $("#list-ch-volume").show();
    forming_tickers_list();
    forming_tickers_list_volume();

    $("#btn-price").hasClass("active") && show_price_panel();
    $("#btn-volume").hasClass("active") && show_volume_panel();

    change_pair(document.querySelector('#pairs').value);
}

export  function change_pair(pair) {

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

function update_top_data(pair) {
    $.ajax({
        url: 'updateTop',
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

async function chart_volume_update() {

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
                url: 'getChangeDelta',
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
                    url: 'getChangeDelta',
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

function forming_tickers_list() {

    var start = performance.now();

    const tbody = document.querySelector("#tbody-price");
    tbody.innerHTML = '';
    const th = document.querySelectorAll("thead[name=thead-price] th");

    const listChPrice = document.querySelector('#list-ch-price');
    const list = document.querySelector('#list');
    const listTop = document.querySelector('#list-top');
    const theadPrice = document.querySelector("thead[name=thead-price]");

    const heads = ['1m', '3m', '15m', '1h', '4h', '1d'];

    // Изменение высоты блоков
    listChPrice.style.height = `${list.clientHeight - listTop.clientHeight}px`;
    document.querySelector("#tbody-price").style.height = `${listChPrice.clientHeight - theadPrice.clientHeight}px`;
    document.querySelector("#table-price").style.marginBottom = '0';


    const btnPairsFavorite = document.querySelector("#btnFavoritePairs");

    var { marketsStat, changePrices } = fetchChangePrices();

    let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];
    let filterPairs = JSON.parse(localStorage.getItem('filterPairs')) || [];
    
    for (let pair in changePrices) {

        // Избранные пары
        if (btnPairsFavorite.classList.contains("active") && !favoritePairs.includes(pair)) {
            continue;
        }
        // Фильтры
        if (!filterPairs.includes(pair)) {
            continue;
        }
        
        let row = tbody.insertRow(-1);
        row.className = "pair-price";

        function createCell(innerHTML, attributeObject, classCell, element, widthColum) {
            let cell = row.insertCell();
            cell.innerHTML = innerHTML;
            for (let key in attributeObject) {
                cell.setAttribute(key, attributeObject[key]);
            }
            cell.classList.add(classCell);
            if (element != null) {
                cell.appendChild(chk);
            }
        }

        // 1 столбец Favorite checkbox
        let chk = document.createElement('input');
        chk.setAttribute('type', 'checkbox');
        chk.setAttribute("name", pair);
        chk.setAttribute('class', 'form-check-input favorite-pair');
        if (favoritePairs.includes(pair)) {
            chk.checked = true;
        }
        createCell('', '', 'price-col1', chk, 'col1');
        // 2 столбец ПАРА
        createCell(pair.split("USDT")[0], { 'name': 'pair' }, 'price-col2', null, 'col2');
        // 3 столбец Volume
        createCell(
            (marketsStat[pair].Volume).toLocaleString('en-US', { maximumFractionDigits: 0, notation: 'compact' }),
            { 'name': 'volume', 'value': marketsStat[pair].Volume }, 'price-col3', null, 'col3');
        // 4 столбец ch1m
        createCell(changePrices[pair][heads[0]]['ChangePercent'].toFixed(2), { 'name': heads[0] }, 'price-col4', null, 'col4');
        // 5 столбец ch3m
        createCell(changePrices[pair][heads[1]]['ChangePercent'].toFixed(2), { 'name': heads[1] }, 'price-col5', null, 'col5');
        // 6 столбец ch15m
        createCell(changePrices[pair][heads[2]]['ChangePercent'].toFixed(2), { 'name': heads[2] }, 'price-col6', null, 'col6');
        // 7 столбец ch1h
        createCell(changePrices[pair][heads[3]]['ChangePercent'].toFixed(2), { 'name': heads[3] }, 'price-col7', null, 'col7');
        // 8 столбец ch4h
        createCell(changePrices[pair][heads[4]]['ChangePercent'].toFixed(2), { 'name': heads[4] }, 'price-col8', null, 'col8');
        // 9 столбец ch12h
        createCell(changePrices[pair][heads[5]]['ChangePercent'].toFixed(2), { 'name': heads[5] }, 'price-col9', null, 'col9');

    };


    // Отображение пар все или избранное
    let checkboxAll = document.querySelectorAll('.favorite-pair');
    checkboxAll.forEach((checkbox, index) => {
        checkbox.addEventListener('change', (e) => {

            let pair = checkbox.getAttribute('name');
            let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];

            if (checkbox.checked) {
                favoritePairs.push(pair);
                localStorage.setItem('favoritePairs', JSON.stringify(favoritePairs));
            } else {
                favoritePairs = favoritePairs.filter((item) => item !== pair);
                localStorage.setItem('favoritePairs', JSON.stringify(favoritePairs));
            }

        });
    });

    // выбор определенной пары
    let rows = tbody.rows;
    for (let row of rows) {
        row.addEventListener("click", () => {
            let pair = row.querySelector('[name="pair"]').innerHTML;
            change_pair(pair + 'USDT');
        });
    };

    // Сортировка таблицы
    const tr = document.querySelectorAll(".pair-price");
    sort_table(tbody, th, tr);


    // Расчет времени выполнения 
    var end = performance.now();
    var time = end - start;
    console.log('Время выполнения forming_tickers_list = ' + time);
}

function forming_tickers_list_volume() {

    var start = performance.now();

    const tbody = document.querySelector("#tbody-delta");
    tbody.innerHTML = '';
    const listChVolume = document.querySelector('#list-ch-volume');
    const list = document.querySelector('#list');
    const listTop = document.querySelector('#list-top');
    const theadDelta = document.querySelector("thead[name=thead-delta]");

    const th = document.querySelectorAll("thead[name=thead-delta] th");
    const btnPairsFavorite = document.querySelector("#btnFavoritePairs");

    // Изменение высоты блоков
    listChVolume.style.height = `${list.clientHeight - listTop.clientHeight}px`;
    document.querySelector("#tbody-delta").style.height = `${listChVolume.clientHeight - theadDelta.clientHeight}px`;
    document.querySelector("#table-delta").style.marginBottom = '0';

    var deltaFast;
    $.ajax({
        url: 'getChDelta',
        async: false,
        method: 'GET',
        cache: false,
        contentType: 'application/json; charset=utf-8',
        processData: false,
        success: function (response) {
            deltaFast = response.DeltaFast;
        },
        error: function (response) {
        },

    });

    let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];
    let filterPairs = JSON.parse(localStorage.getItem('filterPairs')) || [];

    let frame;
    document.querySelectorAll('.btnFrame').forEach(function (element) {
        if (element.classList.contains('active')) {
            frame = element.innerText;
        }
    });

    // для изменения widht по самому широкому стобцу
    //let maxWidths = { 'col1': 0, 'col2': 0, 'col3': 0, 'col4': 0, 'col5': 0, 'col6': 0, 'col7': 0, 'col8': 0, }

    for (let pair in deltaFast) {

        // Избранные пары
        if (btnPairsFavorite.classList.contains("active") && !favoritePairs.includes(pair)) {
            continue;
        }
        // Фильтры
        if (!filterPairs.includes(pair)) {
            continue;
        }

        let row = tbody.insertRow(-1);
        row.className = "pair-delta";

        function createCell(innerHTML, nameCell, classCell, element, widthColum) {
            let cell = row.insertCell();
            cell.innerHTML = innerHTML;
            cell.setAttribute("name", nameCell);
            cell.classList.add(classCell);
            if (element != null) {
                cell.appendChild(chk);
            }
            //maxWidths[widthColum] = Math.max(maxWidths[widthColum], cell.clientWidth);
        }

        // 1 столбец Favorite checkbox
        let chk = document.createElement('input');
        chk.setAttribute('type', 'checkbox');
        chk.setAttribute("name", pair);
        chk.setAttribute('class', 'form-check-input favorite-pair');
        if (favoritePairs.includes(pair)) {
            chk.checked = true;
        }
        createCell('', '', 'delta-col1', chk, 'col1')
        // 2 столбец ПАРА
        createCell(pair.split("USDT")[0], 'pair', 'delta-col2', null, 'col2')
        // 3 столбец Volume
        createCell(deltaFast[pair][frame]["Volume"].toFixed(2), 'volume', 'delta-col3', null, 'col3')
        // 4 столбец VolumeBuy
        createCell(deltaFast[pair][frame]["VolumeBuy"].toFixed(2), 'volume-buy', 'delta-col4', null, 'col4')
        // 5 столбец VolumeAsk
        createCell(deltaFast[pair][frame]["VolumeAsk"].toFixed(2), 'volume-ask', 'delta-col5', null, 'col5')
        // 6 столбец Trades
        createCell(deltaFast[pair][frame]["Trades"].toFixed(2), 'trades', 'delta-col6', null, 'col6')
        // 7 столбец TradesBuy
        createCell(deltaFast[pair][frame]["TradesBuy"].toFixed(2), 'trades-buy', 'delta-col7', null, 'col7')
        // 8 столбец TradesAsk
        createCell(deltaFast[pair][frame]["TradesAsk"].toFixed(2), 'trades-ask', 'delta-col8', null, 'col8')

    };

    // requestAnimationFrame(() => {
    //     // Установить ширину столбцов таблицы, основываясь на самой широкой ячейке в каждом столбце.
    //     for (const colName in maxWidths) {

    //         const th_col_width = document.querySelector(`thead[name=thead-delta] .delta-${colName}`);
    //         const colWidth = Math.max(maxWidths[colName], th_col_width.clientWidth);
    //         document.querySelectorAll(`.delta-${colName}`).forEach(cell => {
    //             cell.style.width = `${colWidth}px`;
    //             cell.style.minWidth = `${colWidth}px`;
    //             cell.style.maxWidth = `${colWidth}px`;
    //         });
    //     }
    // });

    // Отображение пар все или избранное
    let checkboxAll = document.querySelectorAll('.favorite-pair');
    checkboxAll.forEach((checkbox, index) => {
        checkbox.addEventListener('change', (e) => {

            let pair = checkbox.getAttribute('name');
            let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];

            if (checkbox.checked) {
                favoritePairs.push(pair);
                localStorage.setItem('favoritePairs', JSON.stringify(favoritePairs));
            } else {
                favoritePairs = favoritePairs.filter((item) => item !== pair);
                localStorage.setItem('favoritePairs', JSON.stringify(favoritePairs));
            }

        });
    });


    // выбор определенной пары
    let rows = tbody.rows;
    for (let row of rows) {
        row.addEventListener("click", () => {
            let pair = row.querySelector('[name="pair"]').innerHTML;
            change_pair(pair + 'USDT');
        });
    };

    const tr = document.querySelectorAll(".pair-delta");
    // Сортировка таблицы
    sort_table(tbody, th, tr);


    // Расчет времени выполнения 
    var end = performance.now();
    var time = end - start;
    console.log('Время выполнения forming_tickers_list_volume = ' + time);
};

function sort_table(tbody, th, tr) {

    let sortDirection;
    // удалить обработчики старые 
    $(th).off();
    th.forEach((col, idx) => {
        $(col).on("click", () => {
            sortDirection = !sortDirection;
            const rowsArrFromNodeList = Array.from(tr);

            // Первый столбец строки
            if (idx > 0) {
                rowsArrFromNodeList.sort((a, b) => {
                    if (a.childNodes[idx].hasAttribute("value")) {
                        return a.childNodes[idx].getAttribute("value") - b.childNodes[idx].getAttribute("value")
                    }
                    return a.childNodes[idx].innerHTML - b.childNodes[idx].innerHTML
                })
                    .forEach((row) => {
                        sortDirection
                            ? tbody.insertBefore(row, tbody.childNodes[tbody.length])
                            : tbody.insertBefore(row, tbody.childNodes[0]);
                    });

            } else {
                rowsArrFromNodeList.sort((a, b) => {
                    return a.childNodes[idx].innerHTML.localeCompare(
                        b.childNodes[idx].innerHTML,
                        "en",
                        { numeric: true, sensitivity: "base" }
                    );
                })
                    .forEach((row) => {
                        sortDirection
                            ? tbody.insertBefore(row, tbody.childNodes[tbody.length])
                            : tbody.insertBefore(row, tbody.childNodes[0]);
                    });
            }
            // // Перемещение к самой первой строке таблицы
            tbody.childNodes[0].scrollIntoView({ behavior: 'smooth', block: 'nearest' });
        });
    });
}

function show_price_panel() {

    $("#group-btn-frame").hide();

    $("#list-ch-price").show();
    $("#list-ch-volume").hide();

    $("#chart-price").show();
    $("#panel-chart-volume").hide();

    $("#panel-chart-orders").hide();


}

function show_volume_panel() {

    $("#group-btn-frame").show();

    $("#list-ch-price").hide();
    $("#list-ch-volume").show();

    $("#chart-price").hide();
    $("#panel-chart-volume").show();

    $("#panel-chart-orders").hide();
}

export function show_chart_orders() {

    $("#chart-price").hide();
    $("#panel-chart-volume").hide();

    $("#panel-chart-orders").show();
}

function show_panel_trade_active() {
    $("#panel-trade-active").show();
    $("#panel-trade-history").hide();
}
function show_panel_trade_history() {
    $("#panel-trade-active").hide();
    $("#panel-trade-history").show();
}

function color_text_profit(number) {
    if (number >= 0) {
        return 'green';
    } else {
        return 'red';
    }
}

function color_side(side) {
    if (side == 'BUY') {
        return 'green';
    } else {
        return 'red';
    }
}

function showTooltip(element, text) {
    var tooltip = document.createElement('div');
    tooltip.className = 'tooltip';
    tooltip.textContent = text;
    document.body.appendChild(tooltip);

    var rect = element.getBoundingClientRect();
    var tooltipWidth = tooltip.offsetWidth;
    var tooltipHeight = tooltip.offsetHeight;

    tooltip.style.left = rect.left + (rect.width / 2) - (tooltipWidth / 2) + 'px';
    tooltip.style.top = rect.top - tooltipHeight - 5 + 'px';
}

function hideTooltip() {
    var tooltip = document.querySelector('.tooltip');
    if (tooltip) {
        tooltip.parentNode.removeChild(tooltip);
    }
}


function get_response_message(response, reload) {
    if (response['err'] != "" && response['err'] != null) {
        alert(response['err']);
        return true
    } else if (response['message'] != "" && response['message'] != null) {
        alert(response['message']);
        if (reload) location.reload();
        return true
    }
    return false
}



function fetchChangePrices() {

    var start = performance.now();
    var marketsStat;
    var changePrices;

    $.ajax({
        url: 'getChPrice',
        async: false,
        method: 'GET',
        cache: false,
        contentType: 'application/json; charset=utf-8',
        processData: false,
        success: function (response) {
            marketsStat = response.MarketsStat;
            changePrices = response.ChangePrices;
        },
        error: function (response) {
            // Обработка ошибки при необходимости
        }
    });
    // Обновления списка отфильтрованных пар
    filters_get_pairs(marketsStat,changePrices)

    // Расчет времени выполнения 
    var end = performance.now();
    var time = end - start;
    console.log('Время выполнения fetchChangePrices = ' + time);

    return { marketsStat: marketsStat, changePrices: changePrices };
}


function filters_get_pairs(marketsStat,changePrices){

    const heads = ['1m', '3m', '15m', '1h', '4h', '1d'];
    let pairs = [];

   // Фильтры 
   let minVolume = localStorage.getItem('minVolume');
   let maxVolume = localStorage.getItem('maxVolume');
   let minCh1d = localStorage.getItem('minCh1d');
   let maxCh1d = localStorage.getItem('maxCh1d'); 

   for (let pair in changePrices) {
    
        let volume = marketsStat[pair].Volume;
        let ch1d = changePrices[pair][heads[5]]['СhangePercent'];

        if ((minVolume != null && volume <= Number(minVolume)) || (maxVolume != null && volume >= Number(maxVolume))) {
            continue;
        }
        // Ch1d
        if ((minCh1d != null && ch1d <= Number(minCh1d)) || (maxCh1d != null && ch1d >= Number(maxCh1d))) {   
            continue;
        }

        pairs.push(pair)
   }
   localStorage.setItem('filterPairs', JSON.stringify(pairs));
}
