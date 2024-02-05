
import { lw_charts_orders, lw_charts_volume, widget_charts } from './charts.js';
import { timeToLocal } from './help.js';

$(function () {

    //#############################################################################  webSocket #############################################################################

    var socket = new WebSocket("ws://localhost:80/ws");
    socket.onopen = function () {
        console.log("connected ws");
    };

    socket.onmessage = function (e) {

        // Здесь сделать проверку что это вообще за данные, пока что есть данные только для ордеров
        let order = JSON.parse(e.data);
        let orderRow = document.querySelector('tr.order-active[value="' + order.ID + '"]');
        let profitElement = orderRow.querySelector('td[name="order-a-profit"]');

        profitElement.textContent = order.Profit.toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' });
        profitElement.style.color = color_text_profit(order.Profit)

    };


    // если возникла ошибка
    socket.onerror = (error) => {
        console.log(`WebSocket Error: ${error}`);
    };

    // если соединение закрыто
    socket.onclose = (event) => {
        console.log("Connection closed");
    };

    //#############################################################################  webSocket #############################################################################

    // Сплывающее уведомление
    $('.toast').toast({ animation: true, autohide: true, delay: 3000 });
    $("#toastMessage").text("");

    // Аткинвые кнопки меню
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
        change_pair(document.querySelector('#pairs').value)
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

        forming_tickers_list();
        forming_tickers_list_volume();

        change_pair(document.querySelector('#pairs').value);

    });

    // Изменение фрейма для tickers volume
    $('.btnFrame').click(function (e) {

        $('.btnFrame').removeClass('active');
        $(this).addClass('active');

        let frame = e.target.innerText;
        forming_tickers_list_volume(frame);
        change_pair(document.querySelector('#pairs').value);
    });

    $('#btn-update-data').click(function (e) {
        e.preventDefault();
        e.target.disabled = true;
        $.ajax({
            url: '/updatefull',
            type: 'POST',
            method: 'POST',
            cache: false,
            contentType: 'application/json; charset=utf-8',
            processData: false,
            success: function (response) {
                update_main_data(
                    response.MarketsStat,
                    response.ChangePrices,
                    response.DeltaFast,
                );
                $("#toastMessage").text("Данные загружены");
                $(".toast").toast("show");
                e.target.disabled = false;
            },
            error: function (response) {
            },

        });
    });
    // Открыть long/short позицию
    $('.btn-trade-deal').click(function (e) {
        e.preventDefault();

        let sideType

        if (e.target.id == "panel-trdae-open-deal") {
            sideType = "buy";
        }
        if (e.target.id == "panel-trdae-close-deal") {
            sideType = "sell";
        }

        let form = {
            pair: document.querySelector('#pairs').value,
            sideType: sideType,
            strategy: document.querySelector('#panel-trade-strategy').value,
            comment: document.querySelector('#panel-trade-comment').value
        };


        $.ajax({
            url: '/openDeal',
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

});


export function forming_page(pairs, marketsStat, changePrices, deltaFast, ordersActive, ordersHistory) {

    show_price_panel();
    show_panel_trade_active();

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

    // выбор текущей пары
    let currentPair = localStorage.getItem('currentPair') || 'BTCUSDT';
    localStorage.setItem('currentPair', currentPair);

    update_main_data(marketsStat, changePrices, deltaFast);

    // TODO Вот этот блок может перенести от сюда ? 
    // Выбор определенного типа графика
    let checkboxes = document.querySelectorAll('[name="change-delta-check"]');
    checkboxes.forEach((checkbox, index) => {
        checkbox.addEventListener('change', (e) => {
            // Сбросить все галочки 
            checkboxes.forEach((checkboxClear, index) => {
                checkboxClear.checked = false;
            });
            checkbox.checked = true;
            chart_volume_update();
        })
    })

    // Формирование panel-trade
    forming_orders_active(ordersActive);
    forming_orders_history(ordersHistory);
}

function update_main_data(marketsStat, changePrices, deltaFast) {

    localStorage.setItem('marketsStat', JSON.stringify(marketsStat));
    localStorage.setItem('changePrices', JSON.stringify(changePrices));
    localStorage.setItem('deltaFast', JSON.stringify(deltaFast));

    let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];
    localStorage.setItem('favoritePairs', JSON.stringify(favoritePairs));

    forming_tickers_list();
    forming_tickers_list_volume();

    change_pair(document.querySelector('#pairs').value);


    // Загаловки 24ch  Volume
    let selectPairs = document.querySelector('#pairs');
    let ch24Top = document.querySelector('#ch24-top');
    ch24Top.innerHTML = (marketsStat[selectPairs.value].Ch24).toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' }) + '%';
    let VolumeTop = document.querySelector('#volume-top');
    VolumeTop.innerHTML = (marketsStat[selectPairs.value].Volume).toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' });

}

function change_pair(pair) {

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
            rowsPrice[i].classList.add('table-tr-not-active');
            if (rowsPrice[i].querySelector("td[name=pair]").innerHTML === pair) {
                rowsPrice[i].classList.add('table-tr-active');
                rowsPrice[i].scrollIntoView({
                    behavior: 'smooth',
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
            rowsVolume[i].classList.add('table-tr-not-active');
            if (rowsVolume[i].querySelector("td[name=pair]").innerHTML === pair) {
                rowsVolume[i].classList.add('table-tr-active');
                rowsVolume[i].scrollIntoView({
                    behavior: 'smooth',
                    block: 'center'
                });
            }
        }
        chart_volume_update();
    }

    update_top_data(pair);
}

function forming_orders_active(orders) {

    localStorage.setItem('ordersActive', JSON.stringify(orders));

    const tbody = document.querySelector("#tbody-trade-active");
    tbody.innerHTML = '';
    const th = document.querySelectorAll("thead[name=trade-active] th");

    if (orders == null) {
        orders = [];
    }

    for (let order of orders) {

        let row = tbody.insertRow(-1);
        row.className = "order-active";
        row.setAttribute("value", order.ID);

        // 1 Col Side
        let cell = row.insertCell();
        cell.innerHTML = order.Side;
        cell.setAttribute("name", "order-a-side");
        // 2 Col Pair
        cell = row.insertCell();
        cell.innerHTML = order.Pair;
        cell.setAttribute("name", "order-a-pair");
        // 3 Col Price
        cell = row.insertCell();
        cell.innerHTML = order.PriceCreated;
        cell.setAttribute("name", "order-a-price");
        // 4 Col Profit
        cell = row.insertCell();
        cell.innerHTML = order.Profit.toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' });
        cell.style.color = color_text_profit(order.Profit)
        cell.setAttribute("name", "order-a-profit");
        // 5 Col TimeCreated
        cell = row.insertCell();
        cell.innerHTML = new Date(order.TimeCreated).toLocaleString("en-GB");
        cell.setAttribute("name", "order-a-timeCreat");
        // 6 Col - закрыть позицию 
        cell = row.insertCell();
        let btnCl = document.createElement('button');
        btnCl.setAttribute("type", 'button');
        btnCl.setAttribute('class', 'btn-close');
        btnCl.setAttribute('name', 'btn-close-position');
        btnCl.setAttribute('value', order.ID);
        cell.appendChild(btnCl);
    };

    // выбор определенной пары
    let rows = tbody.rows;
    for (let row of rows) {
        row.addEventListener("click", () => {
            let pair = row.querySelector('[name="order-a-pair"]').innerHTML;
            change_pair(pair);
            show_chart_orders();
            chart_frome_orders_update('Active');

        });
    };

    // Закрытие позиции
    let btnsClose = document.querySelectorAll('.btn-close');
    btnsClose.forEach((btn, index) => {
        btn.addEventListener('click', (e) => {
            e.preventDefault();

            $.ajax({
                url: '/closeDeal',
                type: 'POST',
                method: 'POST',
                cache: false,
                contentType: ' text/html; charset=utf-8',
                processData: false,
                data: e.target.value,
                success: function (orders) {
                    forming_orders_active(orders.OrdersActive);
                    forming_orders_history(orders.OrdersHistory);
                },
                error: function (response) {
                    console.log(response);
                },

            });


        });
    });




}

function forming_orders_history(orders) {


    localStorage.setItem('ordersHistory', JSON.stringify(orders));

    const tbody = document.querySelector("#tbody-trade-history");
    tbody.innerHTML = '';
    const th = document.querySelectorAll("thead[name=trade-history] th");

    if (orders == null) {
        orders = [];
    }

    for (let order of orders) {

        let row = tbody.insertRow(-1);
        row.className = "order-history d-flex align-items-center";
        row.setAttribute("value", order.ID);

        // 1 Col Side
        let cell = row.insertCell();
        cell.innerHTML = order.Side;
        cell.setAttribute("name", "order-h-side");
        // 2 Col Pair
        cell = row.insertCell();
        cell.innerHTML = order.Pair;
        cell.setAttribute("name", "order-h-pair");
        // 3 Col - Price
        cell = row.insertCell();
        cell.innerHTML = `${order.PriceCreated} <br> ${order.Price}`;
        cell.setAttribute("name", "order-h-pair");
        // 4 Col TimeCreated
        cell = row.insertCell();
        cell.innerHTML = `${new Date(order.TimeCreated).toLocaleString("en-GB")} <br> ${new Date(order.Time).toLocaleString("en-GB")}`;
        cell.setAttribute("name", "order-h-timeCreat");
        // 5 Col - профит 
        cell = row.insertCell();
        cell.innerHTML = order.Profit.toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' });
    };

    // выбор определенной пары
    let rows = tbody.rows;
    for (let row of rows) {
        row.addEventListener("click", () => {
            let pair = row.querySelector('[name="order-h-pair"]').innerHTML;
            change_pair(pair);
            show_chart_orders();
            chart_frome_orders_update('History');

        });
    };

}

function update_top_data(pair) {
    $.ajax({
        url: '/updateTop',
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

function chart_volume_update() {

    let pair = document.querySelector('#pairs');
    let frames = document.querySelectorAll('.btnFrame');
    let frame;
    let checboxType

    let checkboxes = document.querySelectorAll('[name="change-delta-check"]');
    checkboxes.forEach((checkbox, index) => {
        if (checkbox.checked) {
            checboxType = checkbox.value
        }
    })

    for (let f of frames) {
        if (f.classList.contains('active')) {
            frame = f;
        }
    }

    let container_chart = document.getElementById('chart-volume');
    let chartWidth = container_chart.clientWidth;

    const chartOptions = {
        layout: {
            textColor: 'black',
            background: { type: 'solid', color: 'white' },
        },
        height: 468,
        width: chartWidth,
    };

    function update_volume_data(pair, frame, checboxType) {
        let request = { Pair: pair, Frame: frame};
        let dataVolume = [];
        $.ajax({
            url: '/getChangeDelta',
            async: false,
            type: 'POST',
            method: 'POST',
            data: JSON.stringify(request),
            cache: false,
            contentType: 'application/json; charset=utf-8',
            processData: false,
            success: function (data) {
                for (let item of data) {
                    dataVolume.push({ time: timeToLocal(new Date(item['Time']) / 1000), value: item[checboxType] })
                }
            },
            error: function (response) {
            },
        });
        return dataVolume

    }

    lw_charts_volume(container_chart, chartOptions, pair, frame, checboxType, update_volume_data);


}

function chart_frome_orders_update(chartType) {

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
    let orders;
    if (chartType === 'Active') {
        orders = JSON.parse(localStorage.getItem('ordersActive')) || [];
    }
    if (chartType === 'History') {
        orders = JSON.parse(localStorage.getItem('ordersHistory')) || [];
    }

    function update_candles(pair, frame) {

        let request = { Pair: pair, Frame: frame };
        let candles = [];
        $.ajax({
            url: '/getChangeDelta',
            async: false,
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
            },
            error: function (response) {
            },
        });
        return candles

    }

    lw_charts_orders(container_chart, chartOptions, pair, orders, update_candles);

}

function forming_tickers_list() {

    const heads = ['ch3m', 'ch15m', 'ch1h', 'ch4h'];
    const tbody = document.querySelector("#tbody-price");
    tbody.innerHTML = '';
    const th = document.querySelectorAll("thead[name=thead-price] th");
    const btnPairsFavorite = document.querySelector("#btnFavoritePairs");

    let changePrices = JSON.parse(localStorage.getItem('changePrices')) || [];
    let marketsStat = JSON.parse(localStorage.getItem('marketsStat')) || [];
    let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];

    for (let item in changePrices) {

        if (btnPairsFavorite.classList.contains("active") && !favoritePairs.includes(item)) {
            continue;
        }

        let row = tbody.insertRow(-1);
        row.className = "pair-price";

        // Favorite checkbox
        let cell = row.insertCell();

        let chk = document.createElement('input');
        chk.setAttribute('type', 'checkbox');
        chk.setAttribute("name", item);
        chk.setAttribute('class', 'form-check-input favorite-pair');

        if (favoritePairs.includes(item)) {
            chk.checked = true;
        }
        cell.appendChild(chk);

        // 1 столбец ПАРА
        cell = row.insertCell();
        cell.innerHTML = item;
        cell.setAttribute("name", "pair");

        // 2 столбец Volume
        cell = row.insertCell();
        cell.innerHTML = (marketsStat[item].Volume).toLocaleString('en-US', { maximumFractionDigits: 0, notation: 'compact' });
        cell.setAttribute("name", "volume");
        cell.setAttribute("value", marketsStat[item].Volume);

        // Остальные столбцы
        for (let j = 0; j < heads.length; j++) {
            let cell = row.insertCell();
            let value = changePrices[item][heads[j]]["СhangePercent"].toFixed(2);
            cell.innerHTML = value;
            cell.setAttribute("name", heads[j]);
        }
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
            change_pair(pair);
        });
    };

    // Сортировка таблицы
    const tr = document.querySelectorAll(".pair-price");
    sort_table(tbody, th, tr);


}

function forming_tickers_list_volume(frame = '1m') {

    const tbody = document.querySelector("#tbody-delta");
    tbody.innerHTML = '';
    const th = document.querySelectorAll("thead[name=thead-delta] th");
    const btnPairsFavorite = document.querySelector("#btnFavoritePairs");

    let deltaFast = JSON.parse(localStorage.getItem('deltaFast')) || [];
    let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];

    for (let item in deltaFast) {

        if (btnPairsFavorite.classList.contains("active") && !favoritePairs.includes(item)) {
            continue;
        }

        let row = tbody.insertRow(-1);
        row.className = "pair-delta";

        // Favorite checkbox
        let cell = row.insertCell();

        let chk = document.createElement('input');
        chk.setAttribute('type', 'checkbox');
        chk.setAttribute("name", item);
        chk.setAttribute('class', 'form-check-input favorite-pair');

        if (favoritePairs.includes(item)) {
            chk.checked = true;
        }
        cell.appendChild(chk);

        // 2 столбец ПАРА
        cell = row.insertCell();
        cell.innerHTML = item;
        cell.setAttribute("name", "pair");

        // 3 столбец Volume
        cell = row.insertCell();
        let value = deltaFast[item][frame]["Volume"].toFixed(2);
        cell.innerHTML = value;
        cell.setAttribute("name", 'volume');
        // 4 столбец Volume
        cell = row.insertCell();
        value = deltaFast[item][frame]["Trades"].toFixed(2);
        cell.innerHTML = value;
        cell.setAttribute("name", 'trades');
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
            change_pair(row.querySelector('[name="pair"]').innerHTML);
        });
    };

    const tr = document.querySelectorAll(".pair-delta");
    // Сортировка таблицы
    sort_table(tbody, th, tr);
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

function show_chart_orders() {

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
    if (Number(number >= 0)) {
        return 'green';
    } else {
        return 'red';
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
