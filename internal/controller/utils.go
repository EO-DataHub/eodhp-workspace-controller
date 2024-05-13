package controller

import "fmt"

type Stack[T any] struct {
	elements []T
}

func (s *Stack[T]) Push(element T) {
	s.elements = append(s.elements, element)
}

func (s *Stack[T]) Pop() (T, error) {
	if len(s.elements) == 0 {
		var zero T
		return zero, fmt.Errorf("Stack is empty")
	}
	element := s.elements[len(s.elements)-1]
	s.elements = s.elements[:len(s.elements)-1]
	return element, nil
}

func (s *Stack[T]) Peek() (T, error) {
	if len(s.elements) == 0 {
		var zero T
		return zero, fmt.Errorf("Stack is empty")
	}
	return s.elements[len(s.elements)-1], nil
}

func (s *Stack[T]) IsEmpty() bool {
	return len(s.elements) == 0
}
