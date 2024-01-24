$(function () {

    // Сплывающее уведомление
    $('.toast').toast({ animation: true, autohide: true, delay: 3000 });
    $("#toastMessage").text("");

    // Аткинвые кнопки меню
    $('.btnMenu').click(function () {
        $('.btnMenu').removeClass('active'); // Удаляем класс 'active' у всех кнопок
        $(this).addClass('active'); // Добавляем класс 'active' текущей кнопке
    });

    // Меню цены
    $('#btn-price').click(function (e) {
        show_price_panel();
        change_pair(document.querySelector('#pairs').value)
    });

    // Меню объема
    $('#btn-volume').click(function (e) {
        show_volume_panel();
        change_pair(document.querySelector('#pairs').value)
    });
    $('#btn-deal').click(function (e) {
        show_deal_panel();
    });
    $('#btn-favorite').click(function (e) {
        show_favorite_panel();
    });

    $('#btn-volume-update').click(function (e) {
        e.preventDefault();
        $.ajax({
            url: '/updatefull',
            type: 'POST',
            method: 'POST',
            cache: false,
            contentType: 'application/json; charset=utf-8',
            processData: false,
            success: function (response) {
                forming_tickers_list_volume();
                $("#toastMessage").text("Данные загружены");
                $(".toast").toast("show");
            },
            error: function (response) {
            },

        });
    });

    $('.btnFrame').click(function (e) {
        e.preventDefault();
        let currentBtn = $(this);
        let allButton = $('.btnFrame');
        for (let but of allButton) {
            but.classList.remove('active');
        }
        currentBtn.addClass('active');


        let frame = e.target.innerText;
        $.ajax({
            url: '/updateframe',
            type: 'POST',
            method: 'POST',
            cache: false,
            contentType: 'application/json; charset=utf-8',
            processData: false,
            success: function (response) {
                forming_tickers_list_volume(frame);
                change_pair(document.querySelector('#pairs').value);
            },
            error: function (response) {
            },
        });
    });


    // Аткинвые кнопки выбора пар
    $('.btnPairs').click(function (e) {

        $('.btnPairs').removeClass('active'); // Удаляем класс 'active' у всех кнопок
        $(this).addClass('active'); // Добавляем класс 'active' текущей кнопке

        if ($('#list-ch-price').css('display') == "block") {
            forming_tickers_list();
        }

        if ($('#list-ch-volume').css('display') == "block") {
            forming_tickers_list_volume();
        }
    
    });


});


function forming_page(pairs, marketsStat, changePrices, deltaFast) {

    show_price_panel();
    show_deal_panel();

    // Select pairs
    let selectPairs = document.querySelector('#pairs');
    let selectPairsList = document.querySelector('#pairslistOptions');
    for (index in pairs) {
        let option = new Option(pairs[index], pairs[index]);
        selectPairsList.prepend(option)
    }
    selectPairs.value = "BTCUSDT";

    // Загаловки 24ch  Volume
    let ch24Top = document.querySelector('#ch24-top');
    ch24Top.innerHTML = (marketsStat[selectPairs.value].Ch24).toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' }) + '%';
    let VolumeTop = document.querySelector('#volume-top');
    VolumeTop.innerHTML = (marketsStat[selectPairs.value].Volume).toLocaleString('ru', { maximumFractionDigits: 2, notation: 'compact' });


    //localStorage.clear();

    localStorage.setItem('marketsStat', JSON.stringify(marketsStat));
    localStorage.setItem('changePrices', JSON.stringify(changePrices));
    localStorage.setItem('deltaFast', JSON.stringify(deltaFast));

    let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];
    localStorage.setItem('favoritePairs', JSON.stringify(favoritePairs));




    forming_tickers_list();
    forming_tickers_list_volume();


    selectPairs.addEventListener('change', (e) => {
        let pair = document.querySelector('#pairs');
        change_pair(pair.value);
    });

    // Один раз при инициализации запускаем 
    change_pair(selectPairs.value);


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

}

function change_pair(pair) {

    document.querySelector('#pairs').value = pair;

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
        chart_price_update(pair);
    }
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

function chart_price_update(pair) {
    new TradingView.widget(
        {
            "height": "500",
            "symbol": "BINANCE:" + pair,
            "interval": "15",
            "timezone": "Europe/Moscow",
            "theme": "Light",
            "style": "1",
            "locale": "ru",
            "toolbar_bg": "#f1f3f6",
            "enable_publishing": false,
            "allow_symbol_change": true,
            "container_id": "tradingview_3418f"
        }
    );
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

    let request = { Pair: pair.value, Frame: frame.innerText };
    $.ajax({
        url: '/getChangeDelta',
        type: 'POST',
        method: 'POST',
        data: JSON.stringify(request),
        cache: false,
        contentType: 'application/json; charset=utf-8',
        processData: false,
        success: function (response) {
            let dataFull = response;
            let dataVolume = [];
            for (let item of dataFull) {
                dataVolume.push({ time: item['Time'], value: item[checboxType] })
            }

            const chartOptions = {
                layout: {
                    textColor: 'black',
                    background: { type: 'solid', color: 'white' },
                },
                height: 500,
            };

            let chart_div = document.getElementById('chart-volume');
            chart_div.innerHTML = '';

            const chart = LightweightCharts.createChart(chart_div, chartOptions);
            chart.applyOptions({
                rightPriceScale: {
                    scaleMargins: {
                        top: 0.1,
                        bottom: 0.1,
                    },
                },
            });

            const lineSeries = chart.addLineSeries({ color: '#2962FF' });
            lineSeries.setData(dataVolume);
            chart.timeScale().fitContent();


        },
        error: function (response) {
        },
    });






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

function forming_tickers_list_volume(frame = '5m') {

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

    $("#list-ch-price").show();
    $("#list-ch-volume").hide();

    $("#chart-price").show();
    $("#panel-chart-volume").hide();
}

function show_volume_panel() {
    $("#list-ch-price").hide();
    $("#list-ch-volume").show();

    $("#chart-price").hide();
    $("#panel-chart-volume").show();
}

function show_deal_panel() {

    $("#smart-trade-deal").show();
    $("#smart-trade-favorite").hide();
}


function show_favorite_panel() {

    $("#smart-trade-deal").hide();
    $("#smart-trade-favorite").show();
}




function check_button_state(allButton, currentBtn) {
    for (let but of allButton) {
        but.classList.remove('active');
    }
    currentBtn.addClass('active');
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
