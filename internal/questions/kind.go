package questions

type Kind string

const (
	Noul   Kind = "noul"
	Choice Kind = "choice"
	Score  Kind = "score"
)

func (k Kind) valid() bool {
	return k == Noul || k == Choice || k == Score
}