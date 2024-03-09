// Package district
// Wrote by yijian on 2024/03/09
package district

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "sort"
    "strconv"
    "strings"
)

type Table struct {
    ProvinceDistrictTable map[uint32]ProvinceDistrict `json:"-"`
    Provinces             []ProvinceDistrict          `json:"provinces,omitempty"`
}

// ProvinceDistrict 省/自治区/直辖市
type ProvinceDistrict struct {
    Code              uint32                  `json:"code"`
    Name              string                  `json:"name"`         // 行政区名称
    Level             uint32                  `json:"level"`        // 行政区级别（1 省/自治区/直辖市，2 市/州/盟，3 县/县级市/旗）
    Municipality      bool                    `json:"municipality"` // 直辖市
    CityDistrictTable map[uint32]CityDistrict `json:"-"`
    Cities            []CityDistrict          `json:"cities"`
}

// CityDistrict 市/州/盟
type CityDistrict struct {
    Code                uint32              `json:"code"`
    Name                string              `json:"name"`        // 行政区名称
    Level               uint32              `json:"level"`       // 行政区级别（1 省/自治区/直辖市，2 市/州/盟，3 县/县级市/旗）
    CountyCity          bool                `json:"county_city"` // 县级市
    CountyDistrictTable map[uint32]District `json:"-"`
    Counties            []District          `json:"counties"`
}

type District struct {
    Code        uint32 `json:"code"`        // 行政区代码
    Name        string `json:"name"`        // 行政区名称
    Level       uint32 `json:"level"`       // 行政区级别（1 省/自治区/直辖市，2 市/州/盟，3 县/县级市/旗）
    Parent      uint32 `json:"parent"`      // 父行政区代码
    Grandparent uint32 `json:"grandparent"` // 父父行政区代码
}

func LoadDistrict(ctx context.Context, filepath string) (*Table, error) {
    var districtTable Table

    // 打开文件
    file, err := os.Open(filepath)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    // 创建一个带缓冲的读取器
    reader := bufio.NewReader(file)

    // 按行读取文件内容
    lineNo := 0
    districtTable.ProvinceDistrictTable = make(map[uint32]ProvinceDistrict)
    for {
        lineNo = lineNo + 1
        line, err := reader.ReadString('\n')
        if err != nil {
            if err == io.EOF {
                break
            }
            return nil, err
        }

        line = strings.Trim(line, "\n")
        line = strings.Trim(line, "\r")
        line = strings.TrimSpace(line)
        if len(line) == 0 {
            continue
        }

        district, err := parseLine(lineNo, line)
        if err != nil {
            return nil, err
        } else {
            if district == nil {
                continue
            }
            //fmt.Println(*district)

            provinceCode := getProvinceDistrictCode(district.Code)
            cityCode := getCityDistrictCode(district.Code)
            if isProvinceDistrict(district.Code) {
                // 省/自治区/直辖市
                provinceDistrict := ProvinceDistrict{
                    Code:              district.Code,
                    Name:              district.Name,
                    Level:             district.Level,
                    CityDistrictTable: make(map[uint32]CityDistrict),
                    Municipality:      isMunicipality(district.Code),
                }
                districtTable.ProvinceDistrictTable[provinceCode] = provinceDistrict
            } else if isCityDistrict(district.Code) {
                // 市/州/盟
                cityDistrict := CityDistrict{
                    Code:                district.Code,
                    Name:                district.Name,
                    Level:               district.Level,
                    CountyDistrictTable: make(map[uint32]District),
                    CountyCity:          false,
                }
                districtTable.ProvinceDistrictTable[provinceCode].CityDistrictTable[cityCode] = cityDistrict
            } else if isCountyDistrict(district.Code) {
                if !isMunicipality(district.Code) {
                    if districtTable.ProvinceDistrictTable[provinceCode].CityDistrictTable[cityCode].CountyDistrictTable == nil {
                        // 省直辖县级市（济源市，河南省直辖县级市）
                        cityDistrict := CityDistrict{
                            Code:                district.Code,
                            Name:                district.Name,
                            Level:               district.Level,
                            CountyDistrictTable: make(map[uint32]District),
                            CountyCity:          true,
                        }
                        districtTable.ProvinceDistrictTable[provinceCode].CityDistrictTable[cityCode] = cityDistrict
                    } else {
                        // 县/县级市/旗
                        districtTable.ProvinceDistrictTable[provinceCode].CityDistrictTable[cityCode].CountyDistrictTable[district.Code] = *district
                    }
                } else {
                    // 直辖市的区县
                    cityDistrict := CityDistrict{
                        Code:                district.Code,
                        Name:                district.Name,
                        Level:               district.Level - 1,
                        CountyDistrictTable: make(map[uint32]District),
                    }
                    districtTable.ProvinceDistrictTable[provinceCode].CityDistrictTable[district.Code] = cityDistrict
                }
            } else {
                return nil, fmt.Errorf("invalid row data: (%d) %s", lineNo, line)
            }
        }
    }

    perfectTable(&districtTable)
    return &districtTable, nil
}

func GenerateJson(districtTable *Table) bool {
    jsonBytes, err := json.Marshal(*districtTable)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Json marshal error: %s.\n", err.Error())
        return false
    }

    filepath := "district.json"
    file, writer := createFile(filepath)
    if file == nil {
        return false
    }
    defer file.Close()

    _, err = writer.WriteString(string(jsonBytes))
    if err != nil {
        fmt.Fprintf(os.Stderr, "Write file://%s error: %s.\n", filepath, err.Error())
        return false
    }
    err = writer.Flush()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Flush file://%s error: %s.\n", filepath, err.Error())
        return false
    }

    return true
}

func GenerateCsv(districtTable *Table, csvDelimiter string) bool {
    filepath := "district.csv"
    file, writer := createFile(filepath)
    if file == nil {
        return false
    }
    defer file.Close()

    for _, provinceDistrict := range districtTable.Provinces {
        for _, cityDistrict := range provinceDistrict.Cities {
            if provinceDistrict.Municipality {
                _, err := writer.WriteString(fmt.Sprintf("%s%s%s\n",
                    provinceDistrict.Name, csvDelimiter,
                    cityDistrict.Name))
                if err != nil {
                    fmt.Fprintf(os.Stderr, "Write file://%s error: %s.\n", filepath, err.Error())
                    return false
                }
            } else {
                for _, countyDistrict := range cityDistrict.Counties {
                    _, err := writer.WriteString(fmt.Sprintf("%s%s%s%s%s\n",
                        provinceDistrict.Name, csvDelimiter,
                        cityDistrict.Name, csvDelimiter,
                        countyDistrict.Name))
                    if err != nil {
                        fmt.Fprintf(os.Stderr, "Write file://%s error: %s.\n", filepath, err.Error())
                        return false
                    }
                }
            }
        }
    }
    err := writer.Flush()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Flush file://%s error: %s.\n", filepath, err.Error())
        return false
    }

    return true
}

func parseLine(lineNo int, line string) (*District, error) {
    // 使用逗号分隔每行数据
    parts := strings.Split(line, ",")
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid row format: (%d) %s", lineNo, line)
    }

    // 解析行政区代码
    code, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 32)
    if err != nil {
        if lineNo == 1 {
            return nil, nil
        }
        if len(parts[0]) == 0 {
            // 西沙区
            return nil, nil
        }
        return nil, fmt.Errorf("invalid district code: (%d) %s (%s)", lineNo, parts[0], line)
    }

    // 解析行政区名称
    name := strings.TrimSpace(parts[1])
    grandparent := (code / 10000) * 10000
    parent := (code / 100) * 100

    // 计算行政区级别
    level := uint32(3)
    if code%10000 == 0 {
        level = 1
    } else if code%100 == 0 {
        level = 2
    }

    return &District{
        Code:        uint32(code),
        Name:        name,
        Level:       level,
        Parent:      uint32(parent),
        Grandparent: uint32(grandparent),
    }, nil
}

func isMunicipality(code uint32) bool {
    provinceCode := (code / 10000) * 10000
    return provinceCode == 110000 || // 北京市
        provinceCode == 310000 || // 上海市
        provinceCode == 120000 || // 天津市
        provinceCode == 500000 // 重庆市
}

func isProvinceDistrict(code uint32) bool {
    return code%10000 == 0
}

func isCityDistrict(code uint32) bool {
    return code%10000 != 0 && code%100 == 0
}

func isCountyDistrict(code uint32) bool {
    return code%10000 != 0 && code%100 != 0
}

func getProvinceDistrictCode(code uint32) uint32 {
    return (code / 10000) * 10000
}

func getCityDistrictCode(code uint32) uint32 {
    return (code / 100) * 100
}

func perfectTable(table *Table) {
    for _, provinceDistrict := range table.ProvinceDistrictTable {
        for _, cityDistrict := range provinceDistrict.CityDistrictTable {
            for _, countyDistrict := range cityDistrict.CountyDistrictTable {
                cityDistrict.Counties = append(cityDistrict.Counties, countyDistrict)
            }
            sort.Slice(cityDistrict.Counties, func(i, j int) bool {
                return cityDistrict.Counties[i].Code < cityDistrict.Counties[j].Code
            })
            provinceDistrict.Cities = append(provinceDistrict.Cities, cityDistrict)
        }
        sort.Slice(provinceDistrict.Cities, func(i, j int) bool {
            return provinceDistrict.Cities[i].Code < provinceDistrict.Cities[j].Code
        })
        table.Provinces = append(table.Provinces, provinceDistrict)
    }
    sort.Slice(table.Provinces, func(i, j int) bool {
        return table.Provinces[i].Code < table.Provinces[j].Code
    })
}

func createFile(filepath string) (*os.File, *bufio.Writer) {
    file, err := os.Create(filepath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Create file://%s error: %s.\n", filepath, err.Error())
        return nil, nil
    }

    return file, bufio.NewWriter(file)
}
