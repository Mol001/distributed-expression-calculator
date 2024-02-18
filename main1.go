package main

import (
	"bufio"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MessagePageData struct {
	Message string
}

var (
	additionSeconds       int
	subtractionSeconds    int
	multiplicationSeconds int
	divisionSeconds       int
)

var len_expr = make(map[string]int)

// var db = make(map[string]string)
var agentConnected = make(map[string]string)

// var agent_1 int = 5
// var agent_2 int = 5
// var perts_expr = []string{}
// var db11 = make(map[int][]string)
var db22 = make(map[int][][]string)
var recvest_id = make(map[string]int)
var mutex sync.Mutex

// обработка базовых операций
func evaluateArithmeticExpression(expr string) int {
	parts := []string{}
	if strings.Contains(expr, "/") {
		timer := time.NewTimer(time.Second * time.Duration(divisionSeconds))
		<-timer.C
		parts = strings.Split(expr, "/")
		num1, _ := strconv.Atoi(parts[0])
		num2, _ := strconv.Atoi(parts[1])
		return num1 / num2
	} else if strings.Contains(expr, "*") {
		timer := time.NewTimer(time.Second * time.Duration(multiplicationSeconds))
		<-timer.C
		parts = strings.Split(expr, "*")
		num1, _ := strconv.Atoi(parts[0])
		num2, _ := strconv.Atoi(parts[1])
		return num1 * num2
	} else if strings.Contains(expr, "-") {
		timer := time.NewTimer(time.Second * time.Duration(subtractionSeconds))
		<-timer.C
		parts = strings.Split(expr, "-")
		num1, _ := strconv.Atoi(parts[0])
		num2, _ := strconv.Atoi(parts[1])
		return num1 - num2
	} else if strings.Contains(expr, "+") {
		timer := time.NewTimer(time.Second * time.Duration(additionSeconds))
		<-timer.C
		parts = strings.Split(expr, "+")
		num1, _ := strconv.Atoi(parts[0])
		num2, _ := strconv.Atoi(parts[1])
		return num1 + num2
	} else {
		log.Fatal("Invalid arithmetic expression")
		return 0
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	mutex.Lock()
	port := conn.LocalAddr().(*net.TCPAddr).Port
	agentConnected[strconv.Itoa(port)] = "processing" // статус сервера говорящий о процессе вычисления
	mutex.Unlock()
	// Читаем выражение из соединения
	reader := bufio.NewReader(conn)
	expr, _ := reader.ReadString('\n')

	result := evaluateArithmeticExpression(strings.TrimSpace(expr))

	// Отправляем результат обратно оркестратору
	_, err := conn.Write([]byte(strconv.Itoa(result) + "\n"))
	if err != nil {
		log.Fatal(err)
	}
	mutex.Lock()
	port1 := conn.LocalAddr().(*net.TCPAddr).Port
	agentConnected[strconv.Itoa(port1)] = "finisd" // статус сервера говорящий о зевершенни последней операции
	mutex.Unlock()
}

// Функция отправляющая разделенные на части выражения агентам,
// организующая их последовательное выполнение и сожраняющая их
func sendToAgent(expr_map map[int][][]string, ex_to_stat string, rec_id int) {
	exprs := expr_map[rec_id]
	for _, expr2 := range exprs {
		go func(expr2 []string, rec_id int) {
			expr := expr2[0] + expr2[1] + expr2[2]
			n_op := expr2[3]
			if strings.Contains(expr, "op") == true {
				if strings.Contains(expr2[0], "op") == true && strings.Contains(expr2[2], "op") == true {
					p1 := expr2[0]
					p2 := expr2[2]
					need_p1 := ""
					need_p2 := ""
					flag1 := false
					for flag1 != true {
						mutex.Lock()
						for _, q := range db22[rec_id] {
							if q[0] == p1 {
								need_p1 = q[1]
							} else if q[0] == p2 {
								need_p2 = q[1]
							}
						}
						mutex.Unlock()
						if need_p1 != "" && need_p2 != "" {
							expr := need_p1 + expr2[1] + need_p2
							mutex.Lock()
							data, _ := ReadFromFile("db.txt")
							agentPort := "8080" // Порт первого агента
							if len(data)%2 == 0 {
								agentPort = "8081" // Порт второго агента
							}
							mutex.Unlock()
							// Соединение с агентом
							conn, err := net.Dial("tcp", "localhost:"+agentPort)
							if err != nil {
								log.Fatal(err)
							}
							defer conn.Close()
							// Отправка выражения агенту
							_, err = conn.Write([]byte(strings.TrimSpace(expr) + "\n"))
							if err != nil {
								log.Println("Error sending expression to agent:", err)
								return
							}
							// Получение результата от агента
							result, err := bufio.NewReader(conn).ReadString('\n')
							if err != nil {
								log.Println("Error reading response from agent:", err)
								return
							}
							result = strings.TrimRight(result, "\n")
							// Сохранение результата в базе данных
							mutex.Lock()
							part := []string{}
							part = append(part, n_op)
							part = append(part, result)
							db22[rec_id] = append(db22[rec_id], part)
							if len(db22[rec_id]) == len_expr[ex_to_stat] {
								err12 := UpdateStatus("status.txt", ex_to_stat, "true", strconv.Itoa(rec_id))
								if err12 != nil {
									log.Fatal(err12)
								}
								err := WriteToFile("db.txt", ex_to_stat, db22[rec_id][len(db22[rec_id])-1][1])
								if err != nil {
									log.Fatal(err)
								}
							}
							mutex.Unlock()
							flag1 = true

						}

					}
				} else if strings.Contains(expr2[0], "op") == true {
					p1 := expr2[0]
					need_p1 := ""
					flag2 := false
					for flag2 != true {
						mutex.Lock()
						for _, q := range db22[rec_id] {
							if q[0] == p1 {
								need_p1 = q[1]
							}
						}
						mutex.Unlock()
						if need_p1 != "" {
							expr := need_p1 + expr2[1] + expr2[2]
							mutex.Lock()
							data, _ := ReadFromFile("db.txt")
							agentPort := "8080" // Порт первого агента
							if len(data)%2 == 0 {
								agentPort = "8081" // Порт второго агента
							}
							mutex.Unlock()
							// Соединение с агентом
							conn, err := net.Dial("tcp", "localhost:"+agentPort)
							if err != nil {
								log.Fatal(err)
							}
							defer conn.Close()
							// Отправка выражения агенту
							_, err = conn.Write([]byte(strings.TrimSpace(expr) + "\n"))
							if err != nil {
								log.Println("Error sending expression to agent:", err)
								return
							}
							// Получение результата от агента
							result, err := bufio.NewReader(conn).ReadString('\n')
							if err != nil {
								log.Println("Error reading response from agent:", err)
								return
							}
							result = strings.TrimRight(result, "\n")
							// Сохранение результата в базе данных
							mutex.Lock()
							part := []string{}
							part = append(part, n_op)
							part = append(part, result)
							db22[rec_id] = append(db22[rec_id], part)
							if len(db22[rec_id]) == len_expr[ex_to_stat] {
								err12 := UpdateStatus("status.txt", ex_to_stat, "true", strconv.Itoa(rec_id))
								if err12 != nil {
									log.Fatal(err12)
								}
								err := WriteToFile("db.txt", ex_to_stat, db22[rec_id][len(db22[rec_id])-1][1])
								if err != nil {
									log.Fatal(err)
								}
							}
							mutex.Unlock()
							flag2 = true
						}
					}
				} else if strings.Contains(expr2[2], "op") == true {
					p2 := expr2[2]
					need_p2 := ""
					flag3 := false
					for flag3 != true {
						mutex.Lock()
						for _, q := range db22[rec_id] {
							if q[0] == p2 {
								need_p2 = q[1]
							}
						}
						mutex.Unlock()
						if need_p2 != "" {
							expr := expr2[0] + expr2[1] + need_p2
							mutex.Lock()
							data, _ := ReadFromFile("db.txt")
							agentPort := "8080" // Порт первого агента
							if len(data)%2 == 0 {
								agentPort = "8081" // Порт второго агента
							}
							mutex.Unlock()
							// Соединение с агентом
							conn, err := net.Dial("tcp", "localhost:"+agentPort)
							if err != nil {
								log.Fatal(err)
							}
							defer conn.Close()
							// Отправка выражения агенту
							_, err = conn.Write([]byte(strings.TrimSpace(expr) + "\n"))
							if err != nil {
								log.Println("Error sending expression to agent:", err)
								return
							}
							// Получение результата от агента
							result, err := bufio.NewReader(conn).ReadString('\n')
							if err != nil {
								log.Println("Error reading response from agent:", err)
								return
							}
							result = strings.TrimRight(result, "\n")
							// Сохранение результата в базе данных
							mutex.Lock()
							part := []string{}
							part = append(part, n_op)
							part = append(part, result)
							db22[rec_id] = append(db22[rec_id], part)
							if len(db22[rec_id]) == len_expr[ex_to_stat] {
								err12 := UpdateStatus("status.txt", ex_to_stat, "true", strconv.Itoa(rec_id))
								if err12 != nil {
									log.Fatal(err12)
								}
								err := WriteToFile("db.txt", ex_to_stat, db22[rec_id][len(db22[rec_id])-1][1])
								if err != nil {
									log.Fatal(err)
								}
							}
							mutex.Unlock()
							flag3 = true
						}
					}
				}
			} else {
				// Определение порта агента для обработки выражения
				mutex.Lock()
				data, _ := ReadFromFile("db.txt")
				agentPort := "8080" // Порт первого агента
				if len(data)%2 == 0 {
					agentPort = "8081" // Порт второго агента
				}
				mutex.Unlock()
				// Соединение с агентом
				conn, err := net.Dial("tcp", "localhost:"+agentPort)
				if err != nil {
					log.Fatal(err)
				}
				defer conn.Close()

				// Отправка выражения агенту
				_, err = conn.Write([]byte(strings.TrimSpace(expr) + "\n"))
				if err != nil {
					log.Println("Error sending expression to agent:", err)
					return
				}

				// Получение результата от агента
				result, err := bufio.NewReader(conn).ReadString('\n')
				if err != nil {
					log.Println("Error reading response from agent:", err)
					return
				}
				result = strings.TrimRight(result, "\n")
				// Сохранение результата в базе данных

				mutex.Lock()
				part := []string{}
				part = append(part, n_op)
				part = append(part, result)
				db22[rec_id] = append(db22[rec_id], part)
				if len(db22[rec_id]) == len_expr[ex_to_stat] {
					err12 := UpdateStatus("status.txt", ex_to_stat, "true", strconv.Itoa(rec_id))
					if err12 != nil {
						log.Fatal(err12)
					}
					err := WriteToFile("db.txt", ex_to_stat, db22[rec_id][len(db22[rec_id])-1][1])
					if err != nil {
						log.Fatal(err)
					}
				}
				mutex.Unlock()
			}
		}(expr2, rec_id)
	}
}

func maxOperand(slice []string) (operand string) {
	var maxStr string
	var maxNum int

	for _, str := range slice {
		match := regexp.MustCompile(`op(\d+)`).FindStringSubmatch(str)
		if len(match) > 1 {
			num, _ := strconv.Atoi(match[1])
			if num > maxNum {
				maxNum = num
				maxStr = str
			}
		}
	}
	return maxStr
}

func sliseContains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func splitExpression(expr string) []string {
	re := regexp.MustCompile(`(\d+|\+|\-|\*|\/)`)
	matches := re.FindAllString(expr, -1)
	return matches
}

// проверка выражения на корректность
func checkArithmeticExpression(s string) bool {
	allowedChars := "0123456789+-*/"
	numStarted := false
	for i, char := range s {
		if !strings.ContainsRune(allowedChars, char) {
			return false
		}
		if char >= '0' && char <= '9' {
			if !numStarted {
				numStarted = true
			}
		} else {
			if numStarted && (char != '+' && char != '-' && char != '*' && char != '/') {
				return false
			}
			if i > 0 && s[i-1] != ' ' && s[i-1] != '0' && s[i-1] != '1' && s[i-1] != '2' && s[i-1] != '3' && s[i-1] != '4' && s[i-1] != '5' && s[i-1] != '6' && s[i-1] != '7' && s[i-1] != '8' && s[i-1] != '9' {
				return false
			}
			numStarted = false
		}
	}
	return true
}

// разбивка полученного выражения на части, которые будут отсылаться агентам
func parseExpr(expr string, ri int) (map_expr map[int][][]string, err string) {
	operation_list := make(map[int][][]string, 0)
	expr2 := splitExpression(expr)
	midl_sl := [][]string{}
	if len(expr2) == 3 {
		sr := []string{}
		sr = append(sr, expr2[0])
		sr = append(sr, expr2[1])
		sr = append(sr, expr2[2])
		sr = append(sr, "op"+strconv.Itoa(1))
		midl_sl = append(midl_sl, sr)
	} else {
		k := 1
		used_index := []string{}
		index := []int{}
		for i, n := range expr2 {
			if n == "*" || n == "/" {
				if sliseContains(index, i-1) == false && sliseContains(index, i) == false && sliseContains(index, i+1) == false {
					sr := []string{}
					sr = append(sr, expr2[i-1])
					sr = append(sr, expr2[i])
					sr = append(sr, expr2[i+1])
					fmt.Println(123, sr)
					used_index = append(used_index, strconv.Itoa(i-1), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i+1), "op"+strconv.Itoa(k))
					sr = append(sr, "op"+strconv.Itoa(k))

					index = append(index, i-1)
					index = append(index, i)
					index = append(index, i+1)

					midl_sl = append(midl_sl, sr)
					k += 1
				} else if sliseContains(index, i-1) == true && sliseContains(index, i) == false && sliseContains(index, i+1) == false {
					sr := []string{}
					fp := ""
					fp_slise := []string{}
					for j := 0; j < len(used_index); j += 2 {
						if strconv.Itoa(i-1) == used_index[j] {
							fp = used_index[j+1]
							fp_slise = append(fp_slise, fp)
						}
					}
					fp = maxOperand(fp_slise)

					sr = append(sr, fp)
					sr = append(sr, expr2[i])
					sr = append(sr, expr2[i+1])

					used_index = append(used_index, strconv.Itoa(i-1), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i+1), "op"+strconv.Itoa(k))
					sr = append(sr, "op"+strconv.Itoa(k))

					index = append(index, i-1)
					index = append(index, i)
					index = append(index, i+1)

					midl_sl = append(midl_sl, sr)
					k += 1
				} else if sliseContains(index, i-1) == false && sliseContains(index, i) == false && sliseContains(index, i+1) == true {
					sr := []string{}
					fp := ""
					fp_slise := []string{}
					for j := 0; j < len(used_index); j += 2 {
						if strconv.Itoa(i+1) == used_index[j] {
							fp = used_index[j+1]
							fp_slise = append(fp_slise, fp)
						}
					}
					fp = maxOperand(fp_slise)

					sr = append(sr, expr2[i-1])
					sr = append(sr, expr2[i])
					sr = append(sr, fp)
					used_index = append(used_index, strconv.Itoa(i-1), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i+1), "op"+strconv.Itoa(k))
					sr = append(sr, "op"+strconv.Itoa(k))

					index = append(index, i-1)
					index = append(index, i)
					index = append(index, i+1)

					midl_sl = append(midl_sl, sr)
					k += 1
				} else if sliseContains(index, i-1) == true && sliseContains(index, i) == false && sliseContains(index, i+1) == true {
					sr := []string{}
					fp := ""
					for j := 0; j < len(used_index); j += 2 {
						if strconv.Itoa(i-1) == used_index[j] {
							fp = used_index[j+1]
							break
						}
					}
					fp1 := ""
					for j := 0; j < len(used_index); j += 2 {
						if strconv.Itoa(i+1) == used_index[j] {
							fp1 = used_index[j+1]
							break
						}
					}

					sr = append(sr, fp)
					sr = append(sr, expr2[i])
					sr = append(sr, fp1)
					used_index = append(used_index, strconv.Itoa(i-1), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i+1), "op"+strconv.Itoa(k))
					sr = append(sr, "op"+strconv.Itoa(k))

					index = append(index, i-1)
					index = append(index, i)
					index = append(index, i+1)

					midl_sl = append(midl_sl, sr)
					k += 1
				}

			}
		}
		for i, n := range expr2 {
			if n == "+" || n == "-" {
				if sliseContains(index, i-1) == false && sliseContains(index, i) == false && sliseContains(index, i+1) == false {
					sr := []string{}
					sr = append(sr, expr2[i-1])
					sr = append(sr, expr2[i])
					sr = append(sr, expr2[i+1])
					used_index = append(used_index, strconv.Itoa(i-1), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i+1), "op"+strconv.Itoa(k))
					sr = append(sr, "op"+strconv.Itoa(k))

					index = append(index, i-1)
					index = append(index, i)
					index = append(index, i+1)
					midl_sl = append(midl_sl, sr)
					k += 1
				} else if sliseContains(index, i-1) == true && sliseContains(index, i) == false && sliseContains(index, i+1) == false {
					sr := []string{}
					fp := ""
					fp_slise := []string{}
					for j := 0; j < len(used_index); j += 2 {
						if strconv.Itoa(i-1) == used_index[j] {
							fp = used_index[j+1]
							fp_slise = append(fp_slise, fp)
						}
					}
					if len(fp_slise) > 1 {
						fp = maxOperand(fp_slise)
					} else {
						fp = maxOperand(fp_slise)
						for _, g := range midl_sl {
							if g[0] == fp || g[2] == fp {
								fp = g[3]
							}
						}
					}

					sr = append(sr, fp)
					sr = append(sr, expr2[i])
					sr = append(sr, expr2[i+1])
					used_index = append(used_index, strconv.Itoa(i-1), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i+1), "op"+strconv.Itoa(k))
					sr = append(sr, "op"+strconv.Itoa(k))

					index = append(index, i-1)
					index = append(index, i)
					index = append(index, i+1)

					midl_sl = append(midl_sl, sr)
					k += 1
				} else if sliseContains(index, i-1) == false && sliseContains(index, i) == false && sliseContains(index, i+1) == true {
					sr := []string{}
					fp := ""
					fp_slise := []string{}
					for j := 0; j < len(used_index); j += 2 {
						if strconv.Itoa(i+1) == used_index[j] {
							fp = used_index[j+1]
							fp_slise = append(fp_slise, fp)
						}
					}
					if len(fp_slise) > 1 {
						fp = maxOperand(fp_slise)
					} else {
						fp = maxOperand(fp_slise)
						for _, g := range midl_sl {
							if g[0] == fp || g[2] == fp {
								fp = g[3]
							}
						}
					}

					sr = append(sr, expr2[i-1])
					sr = append(sr, expr2[i])
					sr = append(sr, fp)
					used_index = append(used_index, strconv.Itoa(i-1), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i+1), "op"+strconv.Itoa(k))
					sr = append(sr, "op"+strconv.Itoa(k))

					index = append(index, i-1)
					index = append(index, i)
					index = append(index, i+1)

					midl_sl = append(midl_sl, sr)
					k += 1
				} else if sliseContains(index, i-1) == true && sliseContains(index, i) == false && sliseContains(index, i+1) == true {
					sr := []string{}
					fp := ""
					fp_slise := []string{}
					for j := 0; j < len(used_index); j += 2 {
						if strconv.Itoa(i-1) == used_index[j] {
							fp = used_index[j+1]
							fp_slise = append(fp_slise, fp)
						}
					}
					if len(fp_slise) > 1 {
						fp = maxOperand(fp_slise)
					} else {
						fp = maxOperand(fp_slise)
						for _, g := range midl_sl {
							if g[0] == fp || g[2] == fp {
								fp = g[3]
							}
						}
					}

					fp1 := ""
					fp_slise1 := []string{}
					for j := 0; j < len(used_index); j += 2 {
						if strconv.Itoa(i+1) == used_index[j] {
							fp1 = used_index[j+1]
							fp_slise1 = append(fp_slise1, fp1)
						}
					}
					if len(fp_slise1) > 1 {
						fp1 = maxOperand(fp_slise1)
					} else {
						fp1 = maxOperand(fp_slise1)
						for _, g := range midl_sl {
							if g[0] == fp1 || g[2] == fp1 {
								fp1 = g[3]
							}
						}
					}
					sr = append(sr, fp)
					sr = append(sr, expr2[i])
					sr = append(sr, fp1)
					used_index = append(used_index, strconv.Itoa(i-1), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i), "op"+strconv.Itoa(k))
					used_index = append(used_index, strconv.Itoa(i+1), "op"+strconv.Itoa(k))
					sr = append(sr, "op"+strconv.Itoa(k))

					index = append(index, i-1)
					index = append(index, i)
					index = append(index, i+1)

					midl_sl = append(midl_sl, sr)
					k += 1
				}
			}
		}
	}
	operation_list[ri] = midl_sl
	return operation_list, "200. Выражение успешно принято, распаршено и принято к обработке"
}

func UpdateStatus(filename, key, value, id string) error {
	// чтение файла
	data, err := GetStatus(filename)
	if err != nil {
		return err
	}

	// обновление значения
	value1 := value + " " + id
	data[key] = value1

	// запись обновлённого обратно
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for k, v := range data {
		_, err = file.WriteString(k + "=" + v + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

func GetStatus(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	data := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "="); idx > 0 {
			key := line[:idx]
			value := line[idx+1:]
			data[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return data, nil
}

func WriteToFile(filename, key, value string) error {
	data, err := ReadFromFile(filename)
	if err != nil {
		return err
	}
	data[key] = value
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for k, v := range data {
		_, err = file.WriteString(k + "=" + v + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

func ReadFromFile(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	data := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "="); idx > 0 {
			key := line[:idx]
			value := line[idx+1:]
			data[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return data, nil
}

func incrementNumberInFile(filename string) (int, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	strContent := strings.TrimSpace(string(content))
	num, err := strconv.Atoi(strContent)
	if err != nil {
		return 0, err
	}
	num++
	newContent := strconv.Itoa(num)
	err = ioutil.WriteFile(filename, []byte(newContent), 0644)
	if err != nil {
		return 0, err
	}
	return num, nil
}

// основная функция работающая с выражением
func Orchestrat(w http.ResponseWriter, r *http.Request) {
	expr := r.URL.Query().Get("expr")
	var message string
	message = "400. Выражение невалидно"
	// проверка выражения
	if checkArithmeticExpression(expr) == true && strings.Contains(expr, " ") == false && expr != "" {
		data1, err := ReadFromFile("db.txt")
		if err != nil {
			log.Fatal(err)
		}
		// проверка выражения на повторение
		repiated_expr := false
		for e, _ := range data1 {
			if e == expr {
				repiated_expr = true
			}
		}
		if repiated_expr == false {
			ri, err := incrementNumberInFile("Reqwest_id.txt")
			if err != nil {
				fmt.Println("Error:", err)
			}

			err12 := UpdateStatus("status.txt", expr, "false", strconv.Itoa(ri))
			if err12 != nil {
				log.Fatal(err12)
			}

			pars, _ := parseExpr(expr, ri)
			err1 := WriteToFile("db.txt", expr, "?")
			if err1 != nil {
				log.Fatal(err1)
			}

			recvest_id[expr] = ri
			len_expr[expr] = len(pars[ri])
			sendToAgent(pars, expr, ri)
			message = "200. Выражение успешно принято, распаршено и принято к обработке"
		} else {
			message = "200. Выражение успешно принято, распаршено и принято к обработке"
		}
	}
	tmpl, err := template.ParseFiles("orchestrat.tmpl")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// вывод сообщения
	data := map[string]string{
		"Message": message,
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

// var fin_expr = make(map[string]int)

// Страница с выражениями
func Storage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("storage.tmpl")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data1, err := ReadFromFile("db.txt")
	if err != nil {
		log.Fatal(err)
	}
	tmpl.Execute(w, data1)
}

func Agents(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("agents.tmpl")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	agentStatus := make(map[string]string)
	for port, connected := range agentConnected {
		agentStatus[port] = connected
	}
	tmpl.Execute(w, agentStatus)
}

// Определение структуры для передачи данных в шаблон HTML
type OperationForm struct {
	AdditionSeconds       int
	SubtractionSeconds    int
	MultiplicationSeconds int
	DivisionSeconds       int
}

func OperationTime(w http.ResponseWriter, r *http.Request) {
	// Проверка метода запроса
	if r.Method == "POST" {
		// Обновление глобальных переменных из POST-запроса
		additionSeconds = getFormValue(r, "addition")
		subtractionSeconds = getFormValue(r, "subtraction")
		multiplicationSeconds = getFormValue(r, "multiplication")
		divisionSeconds = getFormValue(r, "division")
	}

	// Создание структуры с текущими значениями
	form := OperationForm{
		AdditionSeconds:       additionSeconds,
		SubtractionSeconds:    subtractionSeconds,
		MultiplicationSeconds: multiplicationSeconds,
		DivisionSeconds:       divisionSeconds,
	}

	// Шаблон HTML для формы
	tmpl, err := template.ParseFiles("operations.tmpl")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Обработка ошибок при рендеринге шаблона
	if err := tmpl.Execute(w, form); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Функция для обновление глобальных переменных из POST-запроса
func getFormValue(r *http.Request, key string) int {
	value := r.FormValue(key)
	intValue, err := strconv.Atoi(value)
	if err != nil {
		fmt.Println("Error parsing form value:", err)
		return 0
	}
	return intValue
}

// главная страница
func serveStaticFile(filename string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filename)
	}
}

func main() {
	// установка значений по умолчанию
	additionSeconds = 5
	subtractionSeconds = 5
	multiplicationSeconds = 5
	divisionSeconds = 5
	agentConnected["8080"] = "waiting"
	agentConnected["8081"] = "waiting"

	// Запуск первого агента
	listener1, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}
	defer listener1.Close()

	go func() {
		for {
			conn, err := listener1.Accept()
			if err != nil {
				log.Fatal(err)
			}
			go handleConnection(conn)
		}
	}()

	// Запуск второго агента
	listener2, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatal(err)
	}
	defer listener2.Close()

	go func() {
		for {
			conn, err := listener2.Accept()
			if err != nil {
				log.Fatal(err)
			}
			go handleConnection(conn)
		}
	}()
	// Проверка на незавершённые процессы при закрытии программы
	status_data, err := GetStatus("status.txt")
	if err != nil {
		log.Fatal(err)
	}
	for key, status := range status_data {
		status_id := strings.Split(status, " ")
		status1 := status_id[0]
		id, _ := strconv.Atoi(status_id[1])
		if status1 == "false" {
			pars, _ := parseExpr(key, id)
			recvest_id[key] = id
			len_expr[key] = len(pars[id])
			sendToAgent(pars, key, id)
		}
	}

	http.HandleFunc("/operations", OperationTime)
	http.HandleFunc("/calculate", Orchestrat)
	http.HandleFunc("/storage", Storage)
	http.HandleFunc("/agents", Agents)
	http.HandleFunc("/", serveStaticFile("index.html"))
	log.Println("Starting HTTP server on localhost:8082")
	go log.Fatal(http.ListenAndServe(":8082", nil))

	select {}
}
